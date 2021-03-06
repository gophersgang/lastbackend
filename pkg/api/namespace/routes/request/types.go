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

package request

import (
	"encoding/json"
	"github.com/lastbackend/lastbackend/pkg/api/context"
	"github.com/lastbackend/lastbackend/pkg/common/errors"
	"github.com/lastbackend/lastbackend/pkg/util/validator"
	"io"
	"io/ioutil"
	"strings"
)

const logLevel = 3

type RequestNamespaceCreateS struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *RequestNamespaceCreateS) DecodeAndValidate(reader io.Reader) *errors.Err {

	var log = context.Get().GetLogger()

	log.V(logLevel).Debug("Request: Namespace: decode and validate data for creating")

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		log.V(logLevel).Errorf("Request: Namespace: decode and validate data for creating err: %s", err.Error())
		return errors.New("namespace").Unknown(err)
	}

	err = json.Unmarshal(body, s)
	if err != nil {
		log.V(logLevel).Errorf("Request: Namespace: convert struct from json err: %s", err.Error())
		return errors.New("namespace").IncorrectJSON(err)
	}

	if s.Name == "" {
		log.V(logLevel).Error("Request: Namespace: parameter name can not be empty")
		return errors.New("namespace").BadParameter("name")
	}

	s.Name = strings.ToLower(s.Name)

	if len(s.Name) < 4 && len(s.Name) > 64 && !validator.IsProjectName(s.Name) {
		log.V(logLevel).Error("Request: Namespace: parameter name not valid")
		return errors.New("namespace").BadParameter("name")
	}

	return nil
}

type RequestNamespaceUpdateS struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *RequestNamespaceUpdateS) DecodeAndValidate(reader io.Reader) *errors.Err {

	var log = context.Get().GetLogger()

	log.V(logLevel).Debug("Request: Namespace: decode and validate data for updating")

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		log.V(logLevel).Errorf("Request: Namespace: decode and validate data for updating err: %s", err.Error())
		return errors.New("namespace").Unknown(err)
	}

	err = json.Unmarshal(body, s)
	if err != nil {
		log.V(logLevel).Errorf("Request: Namespace: convert struct from json err: %s", err.Error())
		return errors.New("namespace").IncorrectJSON(err)
	}

	if s.Name == "" {
		log.V(logLevel).Error("Request: Namespace: parameter name can not be empty")
		return errors.New("namespace").BadParameter("name")
	}

	s.Name = strings.ToLower(s.Name)

	if len(s.Name) < 4 && len(s.Name) > 64 && !validator.IsProjectName(s.Name) {
		log.V(logLevel).Error("Request: Namespace: parameter name not valid")
		return errors.New("namespace").BadParameter("name")
	}

	return nil
}
