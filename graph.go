/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"fmt"
	"sort"
	"strings"
)

func (t *container) Graph() string {
	var sb strings.Builder
	sb.WriteString("digraph glue {\n")
	sb.WriteString("    rankdir=LR;\n")

	type edge struct {
		from string
		to   string
	}

	seen := make(map[edge]bool)
	var edges []edge

	for _, beans := range t.core {
		for _, b := range beans {
			fromName := beanGraphName(b)

			for _, dep := range b.dependencies {
				toName := beanGraphName(dep)
				e := edge{from: fromName, to: toName}
				if !seen[e] {
					seen[e] = true
					edges = append(edges, e)
				}
			}

			for _, fd := range b.factoryDependencies {
				if fd.factory != nil && fd.factory.bean != nil {
					toName := beanGraphName(fd.factory.bean)
					e := edge{from: fromName, to: toName}
					if !seen[e] {
						seen[e] = true
						edges = append(edges, e)
					}
				}
			}
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})

	for _, e := range edges {
		sb.WriteString(fmt.Sprintf("    %q -> %q;\n", e.from, e.to))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func beanGraphName(b *bean) string {
	if b.qualifier != "" {
		return b.qualifier
	}
	return b.name
}
