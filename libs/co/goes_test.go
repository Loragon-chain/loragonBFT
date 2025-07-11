// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package co_test

import (
	"testing"

	"github.com/Loragon-chain/loragonBFT/libs/co"
)

func TestGoes(t *testing.T) {
	var g co.Goes
	g.Go(func() {})
	g.Go(func() {})
	g.Wait()

	<-g.Done()
}
