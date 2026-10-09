// SPDX-FileCopyrightText: 2018 gabrie30 and the gabrie30/ghorg contributors
// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package scm

import (
	"time"

	"github.com/briandowns/spinner"
)

var spinningSpinner *spinner.Spinner

func init() {
	spinningSpinner = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
}
