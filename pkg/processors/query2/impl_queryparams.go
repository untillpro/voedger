/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Michael Saigachenko
 */
package query2

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

func ParseQueryParams(params map[string]string) (*QueryParams, error) {
	qp := &QueryParams{
		Constraints: nil,
		Argument:    nil,
	}

	constraints := &Constraints{
		Order:   []string{},
		Include: []string{},
		Keys:    []string{},
		Where:   make(map[string]interface{}),
	}

	// Parse "order"
	if order, exists := params["order"]; exists && order != "" {
		constraints.Order = strings.Split(order, ",")
	}

	// Parse "limit"
	if limit, exists := params["limit"]; exists && limit != "" {
		parsedLimit, err := strconv.Atoi(limit)
		if err != nil {
			return nil, errors.New("invalid 'limit' parameter")
		}
		constraints.Limit = parsedLimit
	}

	// Parse "skip"
	if skip, exists := params["skip"]; exists && skip != "" {
		parsedSkip, err := strconv.Atoi(skip)
		if err != nil {
			return nil, errors.New("invalid 'skip' parameter")
		}
		constraints.Skip = parsedSkip
	}

	// Parse "include"
	if include, exists := params["include"]; exists && include != "" {
		constraints.Include = strings.Split(include, ",")
	}

	// Parse "keys"
	if keys, exists := params["keys"]; exists && keys != "" {
		constraints.Keys = strings.Split(keys, ",")
	}

	// Parse "where"
	if where, exists := params["where"]; exists && where != "" {
		var whereMap map[string]interface{}
		if err := json.Unmarshal([]byte(where), &whereMap); err != nil {
			return nil, errors.New("invalid 'where' parameter")
		}
		constraints.Where = whereMap
	}

	// Parse "arg"
	if arg, exists := params["arg"]; exists && arg != "" {
		var argMap map[string]interface{}
		if err := json.Unmarshal([]byte(arg), &argMap); err != nil {
			return nil, errors.New("invalid 'arg' parameter")
		}
		qp.Argument = argMap
	}

	// Set Constraints if any constraint exists
	if len(constraints.Order) > 0 || constraints.Limit > 0 || constraints.Skip > 0 || len(constraints.Include) > 0 || len(constraints.Keys) > 0 || len(constraints.Where) > 0 {
		qp.Constraints = constraints
	}

	return qp, nil
}
