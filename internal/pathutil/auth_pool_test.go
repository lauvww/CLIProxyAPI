package pathutil

import "testing"

func TestNormalizeCompareKeyWindowsVariants(t *testing.T) {
	t.Parallel()

	left := `D:\CLIProxyAPI\Auth\Pool-A\`
	right := `d:/cliproxyapi/auth/./Pool-A`
	if NormalizeCompareKey(left) != NormalizeCompareKey(right) {
		t.Fatalf("NormalizeCompareKey mismatch: %q vs %q", NormalizeCompareKey(left), NormalizeCompareKey(right))
	}
}

func TestIsWithinScopeMatchesFilesInsidePool(t *testing.T) {
	t.Parallel()

	scope := `D:\CLIProxyAPI\Auth\Pool-A`
	filePath := `d:/cliproxyapi/auth/pool-a/users/token.json`
	if !IsWithinScope(filePath, scope) {
		t.Fatalf("expected %q to be within %q", filePath, scope)
	}
	if IsWithinScope(`D:\CLIProxyAPI\Auth\Pool-B\token.json`, scope) {
		t.Fatalf("did not expect foreign pool path to match scope")
	}
}
