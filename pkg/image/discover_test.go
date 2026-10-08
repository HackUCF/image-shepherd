package image

import (
	"regexp"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.23.10", "3.23.9", 1},
		{"3.23.5", "3.23.5", 0},
		{"43.20260217.3.1", "43.20260413.3.2", -1},
		{"10.0.20251103-0", "10.0.20250628-0", 1},
		{"3.23", "3.23.0", 0},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPickRelativeToIndex(t *testing.T) {
	d := Discover{
		Index: "https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/cloud/",
		Match: `generic_alpine-(3\.23\.\d+)-x86_64-uefi-cloudinit-r0\.qcow2`,
	}
	body := `<a href="generic_alpine-3.23.9-x86_64-uefi-cloudinit-r0.qcow2">
<a href="generic_alpine-3.23.10-x86_64-uefi-cloudinit-r0.qcow2">
<a href="generic_alpine-3.23.10-x86_64-uefi-cloudinit-r0.qcow2.sha512">
<a href="generic_alpine-3.24.1-x86_64-uefi-cloudinit-r0.qcow2">`
	got, err := d.pick(regexp.MustCompile(d.Match), body)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/cloud/generic_alpine-3.23.10-x86_64-uefi-cloudinit-r0.qcow2"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestPickURLTemplate(t *testing.T) {
	d := Discover{
		Index: "https://builds.coreos.fedoraproject.org/prod/streams/stable/builds/builds.json",
		Match: `"id":\s*"(43\.\d+\.\d+\.\d+)"`,
		URL:   "https://example.test/builds/{version}/fcos-{version}.qcow2.xz",
	}
	body := `{"builds": [{"id": "44.20260913.3.2"}, {"id": "43.20260413.3.2"}, {"id": "43.20260217.3.1"}]}`
	got, err := d.pick(regexp.MustCompile(d.Match), body)
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.test/builds/43.20260413.3.2/fcos-43.20260413.3.2.qcow2.xz"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestPickNoMatch(t *testing.T) {
	d := Discover{Index: "https://x.test/", Match: `foo-(\d+)`}
	if _, err := d.pick(regexp.MustCompile(d.Match), "nothing here"); err == nil {
		t.Error("expected error when nothing matches")
	}
}
