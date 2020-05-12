//
// Copyright (c) 2020 SSH Communications Security Inc.
//
// All rights reserved.
//

package gcloudwebhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/PaesslerAG/jsonpath"
)

var (
	errInvalidData = errors.New("invalid event data")
)

func Jira(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		Errorf(w, http.StatusMethodNotAllowed, "%s", r.Method)
		return
	}
	values := r.URL.Query()
	for k, v := range values {
		log.Printf("Query[%s]=%v", k, v)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		Errorf(w, http.StatusInternalServerError,
			"error reading request body: %s", err)
		return
	}

	ticket, from, to, err := getAssignee(data)
	if err != nil {
		Errorf(w, http.StatusInternalServerError,
			"failed to process request data: %s", err)
		return
	}

	log.Printf("Ticket %s: %s => %s\n", ticket, from, to)

	w.WriteHeader(http.StatusOK)
}

func getAssignee(data []byte) (string, string, string, error) {
	var v interface{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return "", "", "", err
	}

	keyRaw, err := jsonpath.Get("$.issue.key", v)
	if err != nil {
		return "", "", "", err
	}
	key, ok := keyRaw.(string)
	if !ok {
		return "", "", "", errInvalidData
	}

	projectRaw, err := jsonpath.Get("$.issue.fields.project.name", v)
	if err != nil {
		return "", "", "", err
	}
	project, ok := projectRaw.(string)
	if !ok {
		return "", "", "", errInvalidData
	}
	if project != "Operations" {
		return "", "", "", errors.New("invalid project name")
	}

	eventTypeRaw, err := jsonpath.Get("$.issue_event_type_name", v)
	if err != nil {
		return "", "", "", err
	}
	eventType, ok := eventTypeRaw.(string)
	if !ok {
		return "", "", "", errInvalidData
	}

	switch eventType {
	case "issue_assigned":
		arrRaw, err := jsonpath.Get(
			`$.changelog.items[? @.fieldId=="assignee"]`, v)
		if err != nil {
			return "", "", "", err
		}
		arr, ok := arrRaw.([]interface{})
		if !ok {
			return "", "", "", errInvalidData
		}
		if len(arr) != 1 {
			return "", "", "", errInvalidData
		}
		values, ok := arr[0].(map[string]interface{})
		if !ok {
			return "", "", "", errInvalidData
		}

		var from string
		if values["fromString"] != nil {
			from = values["fromString"].(string)
		}
		var to string
		if values["toString"] != nil {
			to = values["toString"].(string)
		}

		return key, from, to, nil

	default:
		return "", "", "", fmt.Errorf("unsupported event type '%s'", eventType)
	}
}
