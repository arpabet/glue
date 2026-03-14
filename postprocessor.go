/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package glue

import (
	"sort"

	"github.com/pkg/errors"
)

func (t *container) applyPostProcessors() error {
	// collect all BeanPostProcessor beans
	var processors []BeanPostProcessor
	for _, beans := range t.core {
		for _, b := range beans {
			if p, ok := b.obj.(BeanPostProcessor); ok {
				processors = append(processors, p)
			}
		}
	}

	if len(processors) == 0 {
		return nil
	}

	// sort by OrderedBean if implemented
	sort.SliceStable(processors, func(i, j int) bool {
		oi, iOrdered := processors[i].(OrderedBean)
		oj, jOrdered := processors[j].(OrderedBean)
		if iOrdered && jOrdered {
			return oi.BeanOrder() < oj.BeanOrder()
		}
		return false
	})

	for _, p := range processors {
		t.logger.Printf("PostProcessor %T\n", p)

		for _, beans := range t.core {
			for _, b := range beans {
				if b.obj == nil {
					continue
				}
				// skip post-processors themselves
				if _, isProcessor := b.obj.(BeanPostProcessor); isProcessor {
					continue
				}

				if err := p.PostProcessBean(b.obj, b.name); err != nil {
					return errors.Errorf("post-processor %T failed for bean '%s': %v", p, b.name, err)
				}
			}
		}
	}

	return nil
}
