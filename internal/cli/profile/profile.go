// Пакет profile реализует команду githerd profile и подкоманды use/create/drop/edit.
package profile

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/Snowbridge/git-herder/internal/appexit"
	"github.com/Snowbridge/git-herder/internal/diag/console"
	"github.com/Snowbridge/git-herder/internal/diag/logging"
	"github.com/Snowbridge/git-herder/internal/profilestore"
	"github.com/Snowbridge/git-herder/internal/redact"
)

// Register добавляет команду profile к корню.
func Register(root *cobra.Command, store *profilestore.Store) {
	ui := console.New()

	cmd := &cobra.Command{
		Use:   "profile [profile-name]",
		Short: "Manage named YAML profiles",
		Long:  "List, show, create, edit, and remove profiles in the user config directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 {
				return showProfile(c, store, ui, args[0])
			}
			return listProfiles(store, ui)
		},
	}

	cmd.AddCommand(
		useCmd(store, ui),
		createCmd(store, ui),
		dropCmd(store, ui),
		editCmd(store, ui),
	)

	root.AddCommand(cmd)
}

func listProfiles(store *profilestore.Store, ui *console.UI) (err error) {
	leave := logging.Enter("dir", store.Dir())
	defer func() { leave("err", err) }()
	names, err := store.ListYAMLNames()
	if err != nil {
		return err
	}
	def, err := store.ReadDefaultName()
	if err != nil {
		return err
	}
	broken, brokenName, err := store.BrokenDefault()
	if err != nil {
		return err
	}
	if broken {
		msg := fmt.Sprintf("Warning: default profile pointer %q has no matching %s.yaml", brokenName, brokenName)
		ui.Warning(msg)
		def = ""
	}
	lines := make([]string, 0, len(names))
	for _, n := range names {
		isDef := def != "" && n == def
		lines = append(lines, ui.HighlightDefault(n, isDef))
	}
	ui.PrintLines(lines)
	return nil
}

func showProfile(_ *cobra.Command, store *profilestore.Store, ui *console.UI, name string) (err error) {
	leave := logging.Enter("name", name)
	defer func() { leave("err", err) }()
	if err := profilestore.ValidateProfileName(name); err != nil {
		ui.Error(err.Error())
		return errExit1(err)
	}
	ok, err := store.Exists(name)
	if err != nil {
		return err
	}
	if !ok {
		ui.Error(fmt.Sprintf("Profile %q not found", name))
		return errExit1(fmt.Errorf("profile not found"))
	}
	doc, err := store.ReadYAMLMap(name)
	if err != nil {
		ui.Error(err.Error())
		return errExit1(err)
	}
	masked := redact.MapYAML(doc)
	out, err := yaml.Marshal(masked)
	if err != nil {
		return err
	}
	ui.PrintLine(strings.TrimSpace(string(out)))
	return nil
}

func useCmd(store *profilestore.Store, ui *console.UI) *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile-name>",
		Short: "Set the default profile pointer",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return useProfile(store, ui, args[0])
		},
	}
}

func createCmd(store *profilestore.Store, ui *console.UI) *cobra.Command {
	var from string
	var edit bool
	cmd := &cobra.Command{
		Use:   "create <profile-name>",
		Short: "Create a new profile file",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return createProfile(c, store, ui, args[0], from, edit)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Copy from an existing profile")
	cmd.Flags().BoolVar(&edit, "edit", false, "Open editor after create")
	return cmd
}

func dropCmd(store *profilestore.Store, ui *console.UI) *cobra.Command {
	return &cobra.Command{
		Use:   "drop <profile-name>",
		Short: "Delete a profile file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return dropProfile(store, ui, args[0])
		},
	}
}

func editCmd(store *profilestore.Store, ui *console.UI) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <profile-name>",
		Short: "Edit a profile in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runEdit(c, store, ui, args[0])
		},
	}
}

func useProfile(store *profilestore.Store, ui *console.UI, name string) (err error) {
	leave := logging.Enter("name", name)
	defer func() { leave("err", err) }()
	if err := profilestore.ValidateProfileName(name); err != nil {
		ui.Error(err.Error())
		return errExit1(err)
	}
	ok, err := store.Exists(name)
	if err != nil {
		return err
	}
	if !ok {
		ui.Error(fmt.Sprintf("Profile %q not found", name))
		return errExit1(fmt.Errorf("profile not found"))
	}
	if err := store.WriteDefault(name); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Default profile set to %q", name))
	return nil
}

func createProfile(cmd *cobra.Command, store *profilestore.Store, ui *console.UI, name, from string, edit bool) (err error) {
	leave := logging.Enter("name", name, "from", from, "edit", edit)
	defer func() { leave("err", err) }()
	if from != "" {
		if err := store.CopyFrom(name, from); err != nil {
			ui.Error(err.Error())
			if errors.Is(err, profilestore.ErrConfig) {
				return errExit1(err)
			}
			return err
		}
	} else if err := store.CreateEmpty(name); err != nil {
		ui.Error(err.Error())
		if errors.Is(err, profilestore.ErrConfig) {
			return errExit1(err)
		}
		return err
	}
	if edit {
		return runEdit(cmd, store, ui, name)
	}
	ui.Success(fmt.Sprintf("Profile %q created", name))
	return nil
}

func dropProfile(store *profilestore.Store, ui *console.UI, name string) (err error) {
	leave := logging.Enter("name", name)
	defer func() { leave("err", err) }()
	if err := profilestore.ValidateProfileName(name); err != nil {
		ui.Error(err.Error())
		return errExit1(err)
	}
	ok, err := store.Exists(name)
	if err != nil {
		return err
	}
	if !ok {
		ui.Error(fmt.Sprintf("Profile %q not found", name))
		return errExit1(fmt.Errorf("profile not found"))
	}
	def, err := store.ReadDefaultName()
	if err != nil {
		return err
	}
	if def == name {
		ui.Error("Cannot drop the current default profile; run profile use with another name first")
		return errExit1(fmt.Errorf("cannot drop default profile"))
	}
	confirm, err := ui.ConfirmDrop(name)
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	if err := store.Delete(name); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Profile %q deleted", name))
	return nil
}

func runEdit(_ *cobra.Command, store *profilestore.Store, ui *console.UI, name string) (err error) {
	leave := logging.Enter("name", name)
	defer func() { leave("err", err) }()
	if err := profilestore.ValidateProfileName(name); err != nil {
		ui.Error(err.Error())
		return errExit1(err)
	}
	ok, err := store.Exists(name)
	if err != nil {
		return err
	}
	if !ok {
		ui.Error(fmt.Sprintf("Profile %q not found", name))
		return errExit1(fmt.Errorf("profile not found"))
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	path, err := store.ProfilePathAbs(name)
	if err != nil {
		return err
	}
	if editor == "" {
		ui.Error("EDITOR is not set")
		ui.PrintLine(path)
		return errExit1(fmt.Errorf("EDITOR is not set"))
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	data, err := store.ReadYAMLBytes(name)
	if err != nil {
		return err
	}
	if err := store.ParseYAML(data); err != nil {
		ui.Error("Profile file is not valid YAML")
		return errExit1(err)
	}
	ui.Success(fmt.Sprintf("Profile %q saved", name))
	return nil
}

func errExit1(err error) error {
	return appexit.WithCode(1, err)
}
