//
// Last.Backend LLC CONFIDENTIAL
// __________________
//
// [2014] - [2017] Last.Backend LLC
// All Rights Reserved.
//
// NOTICE:  All information contained herein is, and remains
// the property of Last.Backend LLC and its suppliers,
// if any.  The intellectual and technical concepts contained
// herein are proprietary to Last.Backend LLC
// and its suppliers and may be covered by Russian Federation and Foreign Patents,
// patents in process, and are protected by trade secret or copyright law.
// Dissemination of this information or reproduction of this material
// is strictly forbidden unless prior written permission is obtained
// from Last.Backend LLC.
//

package runtime

import (
	"fmt"
	"github.com/lastbackend/lastbackend/pkg/agent/context"
	"github.com/lastbackend/lastbackend/pkg/agent/events"
	"github.com/lastbackend/lastbackend/pkg/apis/types"
	"github.com/satori/go.uuid"
	"time"
)

const ContainerRestartTimeout = 10 // seconds
const ContainerStopTimeout = 10    // seconds

type Task struct {
	id string

	close chan bool
	done  chan bool

	meta  types.PodMeta
	state types.PodState
	spec  types.PodSpec

	pod *types.Pod
}

func (t *Task) exec() {

	var (
		log = context.Get().GetLogger()
		pod = context.Get().GetStorage().Pods()
	)

	t.pod.State.Provision = true
	t.pod.Spec.State = t.spec.State

	events.SendPodState(t.pod)

	defer func() {
		t.pod.State.Provision = false
		t.pod.State.Ready = true

		pod.SetPod(t.pod)
		events.SendPodState(t.pod)
		log.Debugf("Task [%s]: done task for pod: %s", t.id, t.pod.Meta.ID)
	}()

	log.Debugf("Task [%s]: start task for pod: %s", t.id, t.pod.Meta.ID)

	// Check spec version
	log.Debugf("Task [%s]: pod spec: %s, new spec: %s", t.id, t.pod.Spec.ID, t.spec.ID)

	if t.spec.State == types.StateDestroy {
		log.Debugf("Task [%s]: pod is marked for deletion: %s", t.id, t.pod.Meta.ID)
		t.containersStateManage()
		return
	}

	if t.spec.ID != t.pod.Spec.ID {
		log.Debugf("Task [%s]: spec is differrent, apply new one: %s", t.id, t.pod.Spec.ID)
		t.pod.Spec.ID = t.spec.ID
		t.imagesUpdate()
		t.containersCreate()
	}

	if len(t.pod.Containers) != len(t.spec.Containers) {
		log.Debugf("Task [%s]: containers count mismatch: %d (%d)", t.id, len(t.pod.Containers), len(t.spec.Containers))
		t.containersCreate()
	}

	// check container state
	t.containersStateManage()
	t.containersRemove()
}

func (t *Task) imagesUpdate() {

	log := context.Get().GetLogger()
	crii := context.Get().GetCri()

	// Check images states
	images := make(map[string]struct{})

	// Get images currently used by this pod
	for _, container := range t.pod.Containers {
		log.Debugf("Task [%s]: add images as used: %s", t.id, container.Image)
		images[container.Image] = struct{}{}
	}

	// Check imaged we need to pull
	for _, spec := range t.spec.Containers {

		// Check image exists and not need to be pulled
		if _, ok := images[spec.Image.Name]; ok {

			log.Debugf("Task [%s]: image exists in prev spec: %s", t.id, spec.Image.Name)
			// Check if image need to be updated
			if !spec.Image.Pull {
				log.Debugf("Task [%s]: image not needed to pull: %s", t.id, spec.Image.Name)
				delete(images, spec.Image.Name)
				continue
			}

			log.Debugf("Task [%s]: image delete from unused: %s", t.id, spec.Image.Name)
			delete(images, spec.Image.Name)
		}

		log.Debugf("Task [%s]: image start pull: %s", t.id, spec.Image.Name)
		crii.ImagePull(&spec.Image)
	}

	// Clean up unused images
	for name := range images {
		log.Debugf("Task [%s]: delete unused images: %s", t.id, name)
		crii.ImageRemove(name)
	}

}

func (t *Task) containersCreate() {

	var (
		err  error
		log  = context.Get().GetLogger()
		crii = context.Get().GetCri()
	)

	log.Debugf("Task [%s]: containers creation process started for pod: %s", t.id, t.pod.Meta.ID)

	// Create new containers
	for id, spec := range t.spec.Containers {
		log.Debugf("Task [%s]: container struct create", t.id)

		c := types.Container{
			Pod:     t.pod.Meta.ID,
			Spec:    id,
			Image:   spec.Image.Name,
			State:   types.ContainerStatePending,
			Created: time.Now(),
		}

		if spec.Labels == nil {
			spec.Labels = make(map[string]string)
		}

		spec.Labels["LB_META"] = fmt.Sprintf("%s/%s/%s", t.pod.Meta.ID, t.pod.Spec.ID, spec.Meta.ID)
		c.ID, err = crii.ContainerCreate(spec)

		if err != nil {
			log.Errorf("Task [%s]: container create error %s", t.id, err.Error())
			c.State = types.ContainerStateError
			c.Status = err.Error()
			break
		}

		log.Debugf("Task [%s]: new container created: %s", t.id, c.ID)
		t.pod.AddContainer(&c)
	}
}

func (t *Task) containersRemove() {

	var (
		log   = context.Get().GetLogger()
		crii  = context.Get().GetCri()
		specs = make(map[string]bool)
	)

	log.Debugf("Task [%s]: start containers removable process for pod: %s", t.id, t.pod.Meta.ID)

	for id := range t.spec.Containers {
		log.Debugf("Task [%s]: add spec %s to valid", t.id, id)
		specs[id] = false
	}

	// Remove old containers
	for _, c := range t.pod.Containers {
		log.Debugf("Task [%s]: container %s has spec %s", t.id, c.ID, c.Spec)
		if _, ok := specs[c.Spec]; !ok || specs[c.Spec] == true {
			log.Debugf("Task [%s]: container %s needs to be removed", t.id, c.ID)
			err := crii.ContainerRemove(c.ID, true, true)
			if err != nil {
				log.Errorf("Task [%s]: container remove error: %s", t.id, err.Error())
			}

			t.pod.DelContainer(c.ID)
			continue
		}

		specs[c.Spec] = true
	}

}

func (t *Task) containersStateManage() {

	log := context.Get().GetLogger()
	crii := context.Get().GetCri()

	defer t.pod.UpdateState()

	log.Debugf("Task [%s]: containers state update from: %s to %s", t.id, t.pod.State.State, t.spec.State)

	if t.spec.State == types.StateDestroy {
		log.Debugf("Task [%s]: pod %s delete %d containers", t.id, t.pod.Meta.ID, len(t.pod.Containers))

		for _, c := range t.pod.Containers {

			err := crii.ContainerRemove(c.ID, true, true)
			c.State = types.StateDestroyed
			c.Status = ""

			if err != nil {
				c.State = types.StateError
				c.Status = err.Error()
			}

			t.pod.DelContainer(c.ID)
		}

		return
	}

	// Update containers states
	if t.spec.State == types.StateStarted  {
		for _, c := range t.pod.Containers {
			log.Debugf("Task [%s]: container: %s try to start", t.id, c.ID)
			err := crii.ContainerStart(c.ID)
			c.State = types.StateStarted
			c.Status = ""

			if err != nil {
				log.Errorf("Task [%s]: container: %s start failed: %s", t.id, c.ID, err.Error())
				c.State = types.StateError
				c.Status = err.Error()
			}
			log.Debugf("Task [%s]: container: %s started", t.id, c.ID)
			t.pod.SetContainer(c)
		}
		return
	}

	if t.spec.State == types.StateStopped {
		timeout := time.Duration(ContainerStopTimeout) * time.Second

		for _, c := range t.pod.Containers {

			err := crii.ContainerStop(c.ID, &timeout)

			c.State = types.StateStopped
			c.Status = ""

			if err != nil {
				log.Errorf("Task [%s]: container: stop error: %s", t.id, err.Error())
				c.State = "error"
				c.Status = err.Error()
			}
			log.Debugf("Task [%s]: container: %s stopped", t.id, c.ID)
			t.pod.SetContainer(c)
		}

		return
	}

	if t.spec.State == types.StateRestarted {
		timeout := time.Duration(ContainerRestartTimeout) * time.Second

		for _, c := range t.pod.Containers {

			err := crii.ContainerRestart(c.ID, &timeout)
			c.State = types.StateStarted
			c.Status = ""

			if err != nil {
				c.State = "error"
				c.Status = err.Error()
			}
			log.Debugf("Task [%s]: container: %s restarted", t.id, c.ID)
			t.pod.SetContainer(c)
		}
		return
	}
}

func (t *Task) finish() {
	t.close <- true
}

func (t *Task) clean() {
	close(t.close)
}

func NewTask(meta types.PodMeta, state types.PodState, spec types.PodSpec, pod *types.Pod) *Task {
	log := context.Get().GetLogger()
	uuid := uuid.NewV4().String()
	log.Debugf("Task [%s]: Create new task for pod: %s", uuid, pod.Meta.ID)
	log.Debugf("Task [%s]: Container spec count: %d", uuid, len(spec.Containers))

	return &Task{
		id:    uuid,
		meta:  meta,
		state: state,
		spec:  spec,
		pod:   pod,
		done:  make(chan bool),
		close: make(chan bool),
	}
}
