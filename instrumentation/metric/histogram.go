//+build !race
// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package metric

import (
	"fmt"
	"github.com/codahale/hdrhistogram"
	"sync/atomic"
	"time"
)

type Histogram struct {
	namedMetric
	histo         *hdrhistogram.WindowedHistogram
	overflowCount int64
}

func newHistogram(name string, max int64, n int) *Histogram {
	return &Histogram{
		namedMetric: namedMetric{name: name},
		histo:       hdrhistogram.NewWindowed(n, 0, max, 1),
	}
}

func (h *Histogram) RecordSince(t time.Time) {
	d := time.Since(t).Nanoseconds()
	if err := h.histo.Current.RecordValue(int64(d)); err != nil {
		atomic.AddInt64(&h.overflowCount, 1)
	}
}

func (h *Histogram) Record(measurement int64) {
	if err := h.histo.Current.RecordValue(measurement); err != nil {
		atomic.AddInt64(&h.overflowCount, 1)
	}
}

func (h *Histogram) String() string {
	var errorRate float64
	histo := h.histo.Current

	if h.overflowCount > 0 {
		errorRate = float64(histo.TotalCount()) / float64(h.overflowCount)
	} else {
		errorRate = 0
	}

	return fmt.Sprintf(
		"metric %s: [min=%f, p50=%f, p95=%f, p99=%f, max=%f, avg=%f, samples=%d, error rate=%f]\n",
		h.name,
		toMillis(histo.Min()),
		toMillis(histo.ValueAtQuantile(50)),
		toMillis(histo.ValueAtQuantile(95)),
		toMillis(histo.ValueAtQuantile(99)),
		toMillis(histo.Max()),
		floatToMillis(histo.Mean()),
		histo.TotalCount(),
		errorRate)
}

func (h *Histogram) Export() exportedMetric {
	histo := h.histo.Merge()

	return &histogramExport{
		h.name,
		toMillis(histo.Min()),
		toMillis(histo.ValueAtQuantile(50)),
		toMillis(histo.ValueAtQuantile(95)),
		toMillis(histo.ValueAtQuantile(99)),
		toMillis(histo.Max()),
		floatToMillis(histo.Mean()),
		histo.TotalCount(),
	}
}

func (h *Histogram) Rotate() {
	h.histo.Rotate()
}
