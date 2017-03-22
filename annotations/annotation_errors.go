/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"errors"
	"fmt"
)

var (
	// ErrMissingAnnotations the metatdata does not contains annotations
	ErrMissingAnnotations = errors.New("metatdata without annotations")

	// ErrInvalidAnnotationName the metatdata doesn't contain a valid annotation name
	ErrInvalidAnnotationName = errors.New("invalid annotation name")

	// ErrInvalidAnnotationContent the metadata annotation content is invalid
	ErrInvalidAnnotationContent = errors.New("invalid annotation content")
)

// NewInvalidAnnotationContent returns a new InvalidContent error
func NewInvalidAnnotationContent(name string, val interface{}) error {
	return InvalidContent{
		Name: fmt.Sprintf("the annotation %v does not contains a valid value (%v)", name, val),
	}
}

// IsMissingAnnotations checks the error type
func IsMissingAnnotations(e error) bool {
	return e == ErrMissingAnnotations
}

// InvalidContent error
type InvalidContent struct {
	Name string
}

func (e InvalidContent) Error() string {
	return e.Name
}

// IsInvalidContent checks the error type
func IsInvalidContent(e error) bool {
	_, ok := e.(InvalidContent)
	return ok
}
