package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new module or project (bindle.json)",
		RunE:  notImplemented("init"),
	}
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <module>[@version]",
		Short: "Add a dependency to bindle.json",
		Args:  cobra.MinimumNArgs(1),
		RunE:  notImplemented("add"),
	}
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Resolve, fetch, restore objects, run migrations, wire *LIBL",
		RunE:  notImplemented("install"),
	}
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Compile module objects in dependency order",
		RunE:  notImplemented("build"),
	}
}

func newPublishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "publish",
		Short: "Package (SAVF) and push to the registry",
		RunE:  notImplemented("publish"),
	}
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Inspect the resolved dependency graph",
		RunE:  notImplemented("list"),
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "tree",
		Short: "Print the dependency tree",
		RunE:  notImplemented("list tree"),
	})
	return cmd
}
