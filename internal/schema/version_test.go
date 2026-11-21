package schema

import (
	"errors"
	"testing"
)

// TestAContractForAnotherVersionNamesWhatItDeclaresAndWhereItIsWritten holds
// the facts a surface needs to say what differs and what to do about it: the
// version the file declares and the line it is on, or that it declares none.
func TestAContractForAnotherVersionNamesWhatItDeclaresAndWhereItIsWritten(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data string
		want *VersionError
	}{
		{
			name: "another version, on the second line",
			data: "# rules\nschema_version = \"1\"\n",
			want: &VersionError{Declared: "1", Line: 2, Supported: supportedVersion},
		},
		{
			name: "written with other spacing",
			data: "\n  schema_version=\"3\"\n[privacy]\n",
			want: &VersionError{Declared: "3", Line: 2, Supported: supportedVersion},
		},
		{
			name: "no version at all",
			data: "[privacy]\nnever_egress_dirs = []\n",
			want: &VersionError{Missing: true, Supported: supportedVersion},
		},
		{name: "the version this reads", data: "schema_version = \"" + supportedVersion + "\"\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkVersion([]byte(tt.data))
			if tt.want == nil {
				if err != nil {
					t.Fatalf("checkVersion = %v, want nil", err)
				}
				return
			}
			got, ok := errors.AsType[*VersionError](err)
			if !ok {
				t.Fatalf("checkVersion = %v, want a *VersionError", err)
			}
			if *got != *tt.want {
				t.Errorf("checkVersion = %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

// TestOnlyAWholeNumberBelowTheVersionThisReadsIsOlder holds the line between a
// rules file written in the format before this one and every other kind of
// mismatch. A newer file, a missing version and a value that is not a number
// are not older, so each keeps the message it had; and "10" is above "2" as a
// number although it sorts below it as text.
func TestOnlyAWholeNumberBelowTheVersionThisReadsIsOlder(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		err  VersionError
		want bool
	}{
		{"the version before this one", VersionError{Declared: "1", Supported: "2"}, true},
		{"a version below any that was written", VersionError{Declared: "0", Supported: "2"}, true},
		{"a newer version", VersionError{Declared: "3", Supported: "2"}, false},
		{"a newer version that sorts below as text", VersionError{Declared: "10", Supported: "2"}, false},
		{"a version with a fraction", VersionError{Declared: "1.0", Supported: "2"}, false},
		{"a negative number", VersionError{Declared: "-1", Supported: "2"}, false},
		{"a word", VersionError{Declared: "one", Supported: "2"}, false},
		{"a number too long to hold", VersionError{Declared: "99999999999999999999", Supported: "2"}, false},
		{"no version at all", VersionError{Missing: true, Supported: "2"}, false},
		{"an empty declaration", VersionError{Supported: "2"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Older(); got != tt.want {
				t.Errorf("%+v.Older() = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestAuthorityRemembersAContractForAnotherVersionWithoutClosingAnything(t *testing.T) {
	t.Parallel()
	cause := &VersionError{Declared: "1", Line: 1, Supported: supportedVersion}
	authority := CaptureAuthority(nil, UnsupportedVersion(cause))
	if got := authority.VersionMismatch(); got != cause {
		t.Errorf("VersionMismatch() = %v, want %v", got, cause)
	}
	if authority.Governed() || !authority.Claim().Trustworthy() || authority.Diagnostic() != "" {
		t.Errorf("a contract for another version governs=%v trustworthy=%v diagnostic=%q, want it read as no contract at all",
			authority.Governed(), authority.Claim().Trustworthy(), authority.Diagnostic())
	}
	if other := CaptureAuthority(nil, Unreadable(cause)).VersionMismatch(); other != nil {
		t.Errorf("an unreadable contract reports %v as a version mismatch", other)
	}
}
