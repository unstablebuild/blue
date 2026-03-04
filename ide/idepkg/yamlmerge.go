// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idepkg

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// loadOrCreateUserConfig reads a YAML file at path into a yaml.Node document.
// If the file does not exist or is empty, it logs a warning and returns an
// empty document containing an empty mapping node.
func loadOrCreateUserConfig(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read user config: %w", err)
	}
	if os.IsNotExist(err) || len(data) == 0 {
		if os.IsNotExist(err) {
			log.Warnf("user config %s does not exist, creating empty config", path)
		} else {
			log.Warnf("user config %s is empty, creating empty config", path)
		}
		return &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: "!!map"},
			},
		}, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal user config: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode, Tag: "!!map"},
			},
		}, nil
	}
	return &doc, nil
}

// mergeYAMLNodes deep-merges src into dst. Both must be mapping nodes.
// For each key in src: if the key does not exist in dst, append it;
// if both values are mappings, recurse; otherwise replace dst's value
// with a clone of src's value.
func mergeYAMLNodes(dst, src *yaml.Node) {
	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return
	}
	mergeMappings(dst, src)
}

func mergeMappings(dst, src *yaml.Node) {
	for i := 0; i < len(src.Content)-1; i += 2 {
		srcKey := src.Content[i]
		srcVal := src.Content[i+1]

		dstIdx := findMappingKey(dst, srcKey.Value)
		if dstIdx < 0 {
			dst.Content = append(dst.Content, cloneNode(srcKey), cloneNode(srcVal))
			continue
		}
		dstVal := dst.Content[dstIdx+1]
		if dstVal.Kind == yaml.MappingNode && srcVal.Kind == yaml.MappingNode {
			mergeMappings(dstVal, srcVal)
		} else {
			dst.Content[dstIdx+1] = cloneNode(srcVal)
		}
	}
}

// findMappingKey returns the index of the key node in a mapping's Content
// slice, or -1 if not found.
func findMappingKey(mapping *yaml.Node, key string) int {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// expandNodeValues walks all scalar nodes in the tree and applies
// os.Expand with the given mapping function. Only string-tagged scalars
// are expanded (int, float, bool, null are skipped).
func expandNodeValues(n *yaml.Node, mapping func(string) string) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode, yaml.MappingNode:
		for _, child := range n.Content {
			expandNodeValues(child, mapping)
		}
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!int", "!!float", "!!bool", "!!null":
			return
		}
		n.Value = os.Expand(n.Value, mapping)
	}
}

// backupUserConfig copies path to path+".backup" before any write.
// It is a no-op if the source file does not exist.
func backupUserConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read for backup: %w", err)
	}
	backupPath := path + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return nil
}

// writeYAMLAtomic writes doc to path atomically. It creates a temp file in
// the same directory, encodes the document, verifies that the written content
// contains all expected keys from expected, then renames.
func writeYAMLAtomic(path string, doc *yaml.Node, expected *yaml.Node) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("close encoder: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}

	// Verify round-trip
	readBack, err := os.ReadFile(tmpName)
	if err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("read back: %w", err)
	}
	var written yaml.Node
	if err := yaml.Unmarshal(readBack, &written); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("unmarshal read back: %w", err)
	}
	if written.Kind != yaml.DocumentNode || len(written.Content) == 0 {
		_ = os.Remove(tmpName)
		return fmt.Errorf("written file has unexpected structure")
	}
	if err := verifyMerge(written.Content[0], expected); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("verification failed: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// verifyMerge recursively walks expected mapping keys and confirms each
// exists in written with matching scalar values.
func verifyMerge(written, expected *yaml.Node) error {
	if expected.Kind != yaml.MappingNode {
		return nil
	}
	if written.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind %d", written.Kind)
	}
	for i := 0; i < len(expected.Content)-1; i += 2 {
		key := expected.Content[i].Value
		expVal := expected.Content[i+1]

		wIdx := findMappingKey(written, key)
		if wIdx < 0 {
			return fmt.Errorf("key %q missing from written config", key)
		}
		wVal := written.Content[wIdx+1]

		switch expVal.Kind { //nolint:exhaustive
		case yaml.MappingNode:
			if err := verifyMerge(wVal, expVal); err != nil {
				return fmt.Errorf("key %q: %w", key, err)
			}
		case yaml.ScalarNode:
			if wVal.Value != expVal.Value {
				return fmt.Errorf("key %q: expected %q, got %q", key, expVal.Value, wVal.Value)
			}
		}
	}
	return nil
}

// cloneNode returns a deep copy of a yaml.Node tree.
func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        n.Kind,
		Style:       n.Style,
		Tag:         n.Tag,
		Value:       n.Value,
		Anchor:      n.Anchor,
		Alias:       cloneNode(n.Alias),
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
		Line:        n.Line,
		Column:      n.Column,
	}
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			clone.Content[i] = cloneNode(child)
		}
	}
	return clone
}
