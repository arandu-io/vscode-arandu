package extension_test

import (
	"os"
	"strings"
	"testing"
)

// TestTheNativeCommandsAreDeclaredAndImplemented keeps the manifest and the
// extension from drifting apart.
//
// A command declared and not registered appears in the palette and reports
// that it does not exist when somebody runs it. One registered and not declared
// cannot be reached at all. Both failures are invisible until a person tries.
func TestTheNativeCommandsAreDeclaredAndImplemented(t *testing.T) {
	var manifest struct {
		Contributes struct {
			Commands []struct {
				Command  string `json:"command"`
				Title    string `json:"title"`
				Category string `json:"category"`
			} `json:"commands"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	declared := map[string]string{}
	for _, command := range manifest.Contributes.Commands {
		declared[command.Command] = command.Title
	}

	source := readFile(t, "src/extension.ts")

	for _, command := range []string{"arandu.native.run", "arandu.native.build", "arandu.native.dev"} {
		title, ok := declared[command]
		if !ok {
			t.Errorf("%s is not declared in the manifest, so nothing can reach it", command)
			continue
		}
		if title == "" {
			t.Errorf("%s has no title, and the palette would list a blank row", command)
		}
		if !strings.Contains(source, `registerCommand("`+command+`"`) {
			t.Errorf("%s is declared and never registered: the palette offers it and it fails when run", command)
		}
	}
}

// TestTheNativeCommandsRunTheCommandsTheyName keeps the adapter from inventing
// a command line of its own.
//
// The arguments live in the contract because this extension translates and does
// not decide: a flag chosen here would be a second opinion about how the tool
// is invoked, and the tool is where that belongs.
func TestTheNativeCommandsRunTheCommandsTheyName(t *testing.T) {
	var contract struct {
		NativeRunArgs      []string `json:"nativeRunArgs"`
		NativeDevArgs      []string `json:"nativeDevArgs"`
		NativeBuildArgs    []string `json:"nativeBuildArgs"`
		NativeTargetMarker string   `json:"nativeTargetMarker"`
	}
	readJSON(t, "src/adapterContract.json", &contract)

	for name, args := range map[string][]string{
		"native:run":   contract.NativeRunArgs,
		"native:dev":   contract.NativeDevArgs,
		"native:build": contract.NativeBuildArgs,
	} {
		if len(args) != 1 || args[0] != name {
			t.Errorf("the contract runs %v where it should run [%s]", args, name)
		}
	}

	if contract.NativeTargetMarker != "cmd/native" {
		t.Errorf("the native target is looked for at %q", contract.NativeTargetMarker)
	}
}

// TestTheNativeButtonAppearsOnlyWhereThereIsATarget keeps a project without one
// from being offered a command that answers with a refusal.
func TestTheNativeButtonAppearsOnlyWhereThereIsATarget(t *testing.T) {
	var manifest struct {
		Contributes struct {
			Menus struct {
				ViewTitle []struct {
					Command string `json:"command"`
					When    string `json:"when"`
				} `json:"view/title"`
			} `json:"menus"`
		} `json:"contributes"`
	}
	readJSON(t, "package.json", &manifest)

	var found bool
	for _, entry := range manifest.Contributes.Menus.ViewTitle {
		if entry.Command != "arandu.native.run" {
			continue
		}
		found = true
		for _, condition := range []string{"arandu.project.selected", "arandu.native.available"} {
			if !strings.Contains(entry.When, condition) {
				t.Errorf("the native button does not require %s: %q", condition, entry.When)
			}
		}
	}
	if !found {
		t.Error("the native button is not on the project map")
	}

	// And the condition has to be answered somewhere, or the button never
	// appears at all -- which is the same as not adding it.
	if !strings.Contains(readFile(t, "src/extension.ts"), `"arandu.native.available"`) {
		t.Error("nothing ever sets arandu.native.available, so the button is never shown")
	}
}

// TestTheNativeGroupHasAnIconOfItsOwn keeps the new group from falling back to
// the folder icon every unnamed group gets.
//
// It also must not borrow the one next to it: Native Screens and Native
// Capabilities sit one row apart and are different things, and two identical
// icons is what makes somebody click the wrong one.
func TestTheNativeGroupHasAnIconOfItsOwn(t *testing.T) {
	source := readFile(t, "src/projectMap.ts")

	if !strings.Contains(source, `"native-screens":`) {
		t.Fatal("the native screens group has no icon, so it falls back to a plain folder")
	}

	screens := iconFor(t, source, "native-screens")
	capabilities := iconFor(t, source, `native-capabilities`)
	if screens == capabilities {
		t.Errorf("Native Screens and Native Capabilities share the icon %q, and sit one row apart", screens)
	}
}

// iconFor reads the icon a group is given in the map's icon table.
func iconFor(t *testing.T, source, group string) string {
	t.Helper()

	marker := `"` + group + `": "`
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("%s has no icon", group)
	}
	rest := source[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("%s has an unterminated icon", group)
	}
	return rest[:end]
}

// readFile reads a file from the repository root.
func readFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(rootPath(t, path))
	if err != nil {
		t.Fatalf("%s could not be read: %v", path, err)
	}
	return string(body)
}

// TestOnlyOneNativeWindowIsOpenedAtATime keeps running and watching from
// producing two.
//
// They are the same application: one opens a window, the other opens a window
// and replaces it on every change. Two at once is not two applications, it is
// two builds racing to overwrite the same binary, with one of the windows
// drawing from whichever lost.
func TestOnlyOneNativeWindowIsOpenedAtATime(t *testing.T) {
	source := readFile(t, "src/extension.ts")

	for _, opener := range []string{"runNative", "watchNative"} {
		body := functionBody(t, source, opener)
		if !strings.Contains(body, "this.nativeTerminal !== undefined") {
			t.Errorf("%s does not check for a window that is already open", opener)
		}
		if !strings.Contains(body, "this.nativeTerminal = terminal") {
			t.Errorf("%s does not record the window it opened, so the next call opens a second", opener)
		}
	}
}

// functionBody answers the text of one method, from its name to the line that
// closes it at the same indentation.
func functionBody(t *testing.T, source, name string) string {
	t.Helper()

	start := strings.Index(source, "private async "+name+"(")
	if start < 0 {
		t.Fatalf("%s is not defined", name)
	}
	rest := source[start:]
	end := strings.Index(rest, "\n  }")
	if end < 0 {
		t.Fatalf("%s is never closed", name)
	}
	return rest[:end]
}
