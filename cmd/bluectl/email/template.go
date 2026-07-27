// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package email

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"net/mail"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"text/template/parse"

	"github.com/unstablebuild/blue/emailprovider"
)

const recipientVariable = "Recipient"

type templateVariables map[string]string

func (v *templateVariables) String() string {
	return ""
}

func (v *templateVariables) Set(value string) error {
	key, substitution, ok := strings.Cut(value, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return errors.New("-X expects key=value")
	}
	if key == recipientVariable {
		return fmt.Errorf("-X variable %q is reserved", recipientVariable)
	}
	if *v == nil {
		*v = make(templateVariables)
	}
	(*v)[key] = substitution
	return nil
}

func parseAddress(value string) (emailprovider.Address, error) {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return emailprovider.Address{}, fmt.Errorf("invalid email address %q: %w", value, err)
	}
	return emailprovider.Address{Name: address.Name, Email: address.Address}, nil
}

func pipeReturnsRoot(pipe *parse.PipeNode, rootAliases map[string]struct{}) bool {
	if pipe == nil || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		return false
	}
	return nodeReturnsRoot(pipe.Cmds[0].Args[0], rootAliases)
}

func nodeReturnsRoot(node parse.Node, rootAliases map[string]struct{}) bool {
	if node == nil || (reflect.ValueOf(node).Kind() == reflect.Pointer && reflect.ValueOf(node).IsNil()) {
		return false
	}
	switch node := node.(type) {
	case *parse.DotNode:
		return true
	case *parse.VariableNode:
		if len(node.Ident) != 1 {
			return false
		}
		_, ok := rootAliases[node.Ident[0]]
		return ok
	case *parse.PipeNode:
		return pipeReturnsRoot(node, rootAliases)
	case *parse.CommandNode:
		return len(node.Args) == 1 && nodeReturnsRoot(node.Args[0], rootAliases)
	case *parse.ChainNode:
		return len(node.Field) == 0 && nodeReturnsRoot(node.Node, rootAliases)
	default:
		return false
	}
}

func collectRootAliases(node parse.Node, rootAliases map[string]struct{}) {
	if node == nil || (reflect.ValueOf(node).Kind() == reflect.Pointer && reflect.ValueOf(node).IsNil()) {
		return
	}
	switch node := node.(type) {
	case *parse.ListNode:
		for _, child := range node.Nodes {
			collectRootAliases(child, rootAliases)
		}
	case *parse.ActionNode:
		collectRootAliases(node.Pipe, rootAliases)
	case *parse.IfNode:
		collectRootAliases(node.Pipe, rootAliases)
		collectRootAliases(node.List, rootAliases)
		collectRootAliases(node.ElseList, rootAliases)
	case *parse.RangeNode:
		collectRootAliases(node.Pipe, rootAliases)
		collectRootAliases(node.List, rootAliases)
		collectRootAliases(node.ElseList, rootAliases)
	case *parse.WithNode:
		collectRootAliases(node.Pipe, rootAliases)
		collectRootAliases(node.List, rootAliases)
		collectRootAliases(node.ElseList, rootAliases)
	case *parse.TemplateNode:
		collectRootAliases(node.Pipe, rootAliases)
	case *parse.PipeNode:
		if pipeReturnsRoot(node, rootAliases) {
			for _, declaration := range node.Decl {
				if len(declaration.Ident) != 0 {
					rootAliases[declaration.Ident[0]] = struct{}{}
				}
			}
		}
		for _, command := range node.Cmds {
			collectRootAliases(command, rootAliases)
		}
	case *parse.CommandNode:
		for _, argument := range node.Args {
			collectRootAliases(argument, rootAliases)
		}
	case *parse.ChainNode:
		collectRootAliases(node.Node, rootAliases)
	}
}

func collectTemplateVariables(
	node parse.Node,
	rootAliases map[string]struct{},
	variables map[string]struct{},
) error {
	if node == nil || (reflect.ValueOf(node).Kind() == reflect.Pointer && reflect.ValueOf(node).IsNil()) {
		return nil
	}
	switch node := node.(type) {
	case *parse.ListNode:
		for _, child := range node.Nodes {
			if err := collectTemplateVariables(child, rootAliases, variables); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return collectTemplateVariables(node.Pipe, rootAliases, variables)
	case *parse.IfNode:
		if err := collectTemplateVariables(node.Pipe, rootAliases, variables); err != nil {
			return err
		}
		if err := collectTemplateVariables(node.List, rootAliases, variables); err != nil {
			return err
		}
		return collectTemplateVariables(node.ElseList, rootAliases, variables)
	case *parse.RangeNode:
		if err := collectTemplateVariables(node.Pipe, rootAliases, variables); err != nil {
			return err
		}
		if err := collectTemplateVariables(node.List, rootAliases, variables); err != nil {
			return err
		}
		return collectTemplateVariables(node.ElseList, rootAliases, variables)
	case *parse.WithNode:
		if err := collectTemplateVariables(node.Pipe, rootAliases, variables); err != nil {
			return err
		}
		if err := collectTemplateVariables(node.List, rootAliases, variables); err != nil {
			return err
		}
		return collectTemplateVariables(node.ElseList, rootAliases, variables)
	case *parse.TemplateNode:
		return collectTemplateVariables(node.Pipe, rootAliases, variables)
	case *parse.PipeNode:
		for _, command := range node.Cmds {
			if err := collectTemplateVariables(command, rootAliases, variables); err != nil {
				return err
			}
		}
	case *parse.CommandNode:
		if len(node.Args) >= 3 {
			identifier, isIndex := node.Args[0].(*parse.IdentifierNode)
			if isIndex && identifier.Ident == "index" {
				if !nodeReturnsRoot(node.Args[1], rootAliases) {
					return errors.New("email template may only index the root variables map")
				}
				key, ok := node.Args[2].(*parse.StringNode)
				if !ok {
					return errors.New("email template must index root variables with a literal key")
				}
				variables[key.Text] = struct{}{}
			}
		}
		for _, argument := range node.Args {
			if err := collectTemplateVariables(argument, rootAliases, variables); err != nil {
				return err
			}
		}
	case *parse.FieldNode:
		if len(node.Ident) != 0 {
			variables[node.Ident[0]] = struct{}{}
		}
	case *parse.VariableNode:
		if len(node.Ident) > 1 {
			if _, ok := rootAliases[node.Ident[0]]; !ok {
				return fmt.Errorf("email template cannot access fields on local variable %s", node.Ident[0])
			}
			variables[node.Ident[1]] = struct{}{}
		}
	case *parse.ChainNode:
		if len(node.Field) != 0 {
			if !nodeReturnsRoot(node.Node, rootAliases) {
				return errors.New("email template cannot access fields on a non-root value")
			}
			variables[node.Field[0]] = struct{}{}
		}
		return collectTemplateVariables(node.Node, rootAliases, variables)
	}
	return nil
}

func validateTemplateVariables(tmpl *template.Template, data map[string]string) error {
	required := make(map[string]struct{})
	rootAliases := map[string]struct{}{"$": {}}
	for {
		aliasesBefore := len(rootAliases)
		for _, associated := range tmpl.Templates() {
			if associated.Tree != nil {
				collectRootAliases(associated.Tree.Root, rootAliases)
			}
		}
		if len(rootAliases) == aliasesBefore {
			break
		}
	}
	for _, associated := range tmpl.Templates() {
		if associated.Tree == nil {
			continue
		}
		if err := collectTemplateVariables(associated.Tree.Root, rootAliases, required); err != nil {
			return err
		}
	}
	missing := make([]string, 0)
	for variable := range required {
		if _, ok := data[variable]; !ok {
			missing = append(missing, variable)
		}
	}
	if len(missing) != 0 {
		slices.Sort(missing)
		return fmt.Errorf("missing template variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func render(path string, recipient emailprovider.Address, variables templateVariables) ([]byte, error) {
	if filepath.Ext(path) != ".tmpl" {
		return nil, fmt.Errorf("email template must have a .tmpl extension: %s", path)
	}
	name := filepath.Base(path)
	tmpl, err := template.New(name).Option("missingkey=error").ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("parse email template: %w", err)
	}

	data := make(map[string]string, len(variables)+1)
	maps.Copy(data, variables)
	data[recipientVariable] = recipient.Email
	if err := validateTemplateVariables(tmpl, data); err != nil {
		return nil, fmt.Errorf("validate email template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rendered, name, data); err != nil {
		return nil, fmt.Errorf("render email template: %w", err)
	}
	return rendered.Bytes(), nil
}
