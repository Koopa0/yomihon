package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestAnswerTypeRequiresExplicitEnumMember(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		declaration string
		want        string
	}{
		{name: "absent"},
		{name: "declared", declaration: `answer_type = "lesson"`, want: "lesson"},
		{name: "another member", declaration: `answer_type = "moc"`, want: "moc"},
		{name: "empty", declaration: `answer_type = ""`},
		{name: "undeclared", declaration: `answer_type = "note"`},
		{name: "case differs", declaration: `answer_type = "Lesson"`},
		{name: "list", declaration: `answer_type = ["lesson"]`},
		{name: "boolean", declaration: `answer_type = true`},
		{name: "number", declaration: `answer_type = 1`},
		{name: "table", declaration: `answer_type = { type = "lesson" }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := answerContract(tt.declaration)
			contract, err := loadContractBytes(t, []byte(data))
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			if got := contract.AnswerType(); got != tt.want {
				t.Errorf("AnswerType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnswerTypeDoesNotReplaceOtherNavigationDeclarations(t *testing.T) {
	t.Parallel()

	for _, declaration := range []string{"", `answer_type = "lesson"`} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()
			contract, err := loadContractBytes(t, []byte(answerContract(declaration)))
			if err != nil {
				t.Fatal(err)
			}
			roles := contract.NavigationRoles()
			if !roles.Available() || !roles.IsPathType("study-path") || !roles.IsMapType("moc") {
				t.Error("answer role changed the declared path/map roles")
			}
			if !contract.JournalDir().Contains("Diary/today.md") {
				t.Error("answer role changed the declared journal directory")
			}
		})
	}
}

func TestAnswerTypeUnknownNavigationKeyClosesRole(t *testing.T) {
	t.Parallel()
	contract, err := loadContractBytes(t, []byte(answerContract("answer_type = \"lesson\"\nanswer_types = [\"moc\"]")))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.AnswerType(); got != "" {
		t.Errorf("AnswerType() = %q despite unknown navigation key", got)
	}
}

func TestAnswerTypeHasNoFabricatedAuthority(t *testing.T) {
	t.Parallel()
	decoded, err := decodeContract([]byte(answerContract(`answer_type = "lesson"`)), policySource{})
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []*Contract{nil, {}, decoded} {
		if got := contract.AnswerType(); got != "" {
			t.Errorf("AnswerType() = %q without a revalidatable contract", got)
		}
	}
}

func TestAnswerTypeFoldsDeclaredWord(t *testing.T) {
	t.Parallel()
	data := strings.ReplaceAll(validContractV1, "lesson", "r\u00e9ponse")
	data = strings.Replace(data, "[navigation]\n", "[navigation]\nanswer_type = \"re\u0301ponse\"\n", 1)
	contract, err := loadContractBytes(t, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := contract.AnswerType(); got != "r\u00e9ponse" {
		t.Errorf("AnswerType() = %q, want NFC enum member", got)
	}
}

func TestAnswerTypeRevocationClosesEveryContractCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contractPath := filepath.Join(root, filepath.FromSlash(ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(answerContract(`answer_type = "lesson"`))
	writeAnswerContract(t, contractPath, data)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("reader.Close(): %v", err)
		}
	})
	contract, err := LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal(err)
	}
	other := *contract
	if got := contract.AnswerType(); got != "lesson" {
		t.Fatalf("AnswerType() = %q before revocation", got)
	}
	writeAnswerContract(t, contractPath, []byte(answerContract("")))
	if got := contract.AnswerType(); got != "" {
		t.Fatalf("AnswerType() = %q after revocation", got)
	}
	writeAnswerContract(t, contractPath, data)
	if got := other.AnswerType(); got != "" {
		t.Errorf("copied AnswerType() = %q after observed revocation", got)
	}
}

func TestAnswerTypeUnreadableSourceClosesOnlyCurrentCall(t *testing.T) {
	t.Parallel()
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	data := []byte(answerContract(`answer_type = "lesson"`))
	writeAnswerContract(t, contractPath, data)
	contract, err := LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(contractPath); err != nil {
		t.Fatal(err)
	}
	if got := contract.AnswerType(); got != "" {
		t.Errorf("AnswerType() = %q with unreadable source", got)
	}
	writeAnswerContract(t, contractPath, data)
	if got := contract.AnswerType(); got != "lesson" {
		t.Errorf("AnswerType() = %q after same source returns", got)
	}
}

func answerContract(declaration string) string {
	return strings.Replace(validContractV1, "[navigation]\n", "[navigation]\njournal_dir = \"Diary\"\n"+declaration+"\n", 1)
}

func writeAnswerContract(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
