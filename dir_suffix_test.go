// SPDX-License-Identifier: MIT

package gocodewalker

import "testing"

func TestIsSuffixDir(t *testing.T) {
	testCases := []struct {
		base   string
		suffix string
		expect bool
	}{
		{
			base:   "",
			suffix: "",
			expect: false,
		},
		{
			base:   "a",
			suffix: "",
			expect: false,
		},
		{
			base:   "",
			suffix: "a",
			expect: false,
		},
		{
			base:   "a",
			suffix: "a",
			expect: true,
		},
		{
			base:   "a/b/c",
			suffix: "a/b/c",
			expect: true,
		},
		{
			base:   "a/b/c",
			suffix: "c",
			expect: true,
		},
		{
			base:   "c",
			suffix: "a/b/c",
			expect: false,
		},
		{
			base:   "a/b/c",
			suffix: "b/c",
			expect: true,
		},
		{
			base:   "/a/b/c",
			suffix: "b/c",
			expect: true,
		},
		{
			base:   "/a/b/c",
			suffix: "/b/c",
			expect: false,
		},
		{
			base:   "a/b/c",
			suffix: "/b/c",
			expect: false,
		},
		{
			base:   "a/b/c",
			suffix: "b",
			expect: false,
		},
		{
			base:   "a/b/c",
			suffix: "a/b",
			expect: false,
		},
		{
			base:   "a/b/c/d",
			suffix: "b/c",
			expect: false,
		},
		{
			base:   "a/bb/c",
			suffix: "b/c",
			expect: false,
		},
		{
			base:   "C:/a/b",
			suffix: "a/b",
			expect: true,
		},
		{
			base:   "C:/a/b",
			suffix: "/a/b",
			expect: false,
		},
		{
			base:   "C:/a/b",
			suffix: "D:/a/b",
			expect: false,
		},
		{
			base:   "b/b/c",
			suffix: "b/b/c/",
			expect: true,
		},
	}
	for _, tc := range testCases {
		res := isSuffixDir(tc.base, tc.suffix)
		if res != tc.expect {
			t.Errorf("base: %s, suffix: %s, got: %v, want: %v", tc.base, tc.suffix, res, tc.expect)
		}
	}
}
