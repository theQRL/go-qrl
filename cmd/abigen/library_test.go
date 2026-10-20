// Copyright 2026 The go-qrl Authors
// This file is part of go-qrl.
//
// go-qrl is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-qrl is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-qrl. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
)

func TestLibraryPlaceholderPattern(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "L",
			want: "8aa64f937099b65a4febc243a5ae0f2d6416bb9e473c30dd29c1ee498fb7c5a8c3bf89f790419cb80c2d3a31e6aa52e8079db9fc85edc2ee01b8618299",
		},
		{
			name: "A:L2",
			want: "622b2f540b6a16ff5db7bea656ad8fcf4fdcdfc3b578264a3fd49241d5cca5ea90da806de44784d6712672a301f4a73ca9c044893868600a9bb498b5eb",
		},
	}

	for _, test := range tests {
		pattern := libraryPlaceholderPattern(test.name)
		if pattern != test.want {
			t.Fatalf("unexpected library placeholder pattern for %q: have %q, want %q", test.name, pattern, test.want)
		}
		if len(pattern) != libraryPlaceholderPatternLength {
			t.Fatalf("unexpected pattern length for %q: have %d, want %d", test.name, len(pattern), libraryPlaceholderPatternLength)
		}
		if len("__$"+pattern+"$__") != 2*common.AddressLength {
			t.Fatalf("placeholder for %q is not address-width: %d", test.name, len("__$"+pattern+"$__"))
		}
		if strings.Contains(pattern, "$") || strings.Contains(pattern, "_") {
			t.Fatalf("pattern for %q should only contain inner hash hex: %q", test.name, pattern)
		}
	}
}
