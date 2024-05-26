// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package list

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	pkgutils "kraftkit.sh/internal/cli/kraft/pkg/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	ukarch "kraftkit.sh/unikraft/arch"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type ListOptions struct {
	All       bool   `long:"all" usage:"Show everything"`
	Arch      string `long:"arch" usage:"Set a specific arhitecture to list for"`
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Limit     int    `long:"limit" short:"l" usage:"Set the maximum number of results" default:"50"`
	Local     bool   `long:"local" usage:"Show local packages only"`
	NoLimit   bool   `long:"no-limit" usage:"Do not limit the number of items to print"`
	Output    string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Plat      string `long:"plat" usage:"Set a specific platform to list for"`
	Remote    bool   `long:"remote" short:"u" usage:"Show remote packages only"`
	ShowApps  bool   `long:"apps" short:"" usage:"Show applications"`
	ShowArchs bool   `long:"archs" short:"M" usage:"Show architectures"`
	ShowCore  bool   `long:"core" short:"C" usage:"Show Unikraft core versions"`
	ShowLibs  bool   `long:"libs" short:"L" usage:"Show libraries"`
	ShowPlats bool   `long:"plats" short:"P" usage:"Show platforms"`
	Update    bool   `long:"update" short:"U" usage:"Update package indexes before listing"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List installed Unikraft component packages",
		Use:     "list [FLAGS] [DIR]",
		Aliases: []string{"ls"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			List installed Unikraft component packages.
		`),
		Example: heredoc.Doc(`
			# List packages
			$ kraft pkg list

			# List all packages
			$ kraft pkg list --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	if opts.Local && opts.Remote {
		return fmt.Errorf("cannot use --local and --remote")
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	cmd.SetContext(ctx)

	return nil
}

func ListAMIs() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	// Create an EC2 service client
	svc := ec2.New(sess)

	// DescribeImages input
	input := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String("amibuilder*")},
			},
		},
	}

	// Call the DescribeImages operation
	result, err := svc.DescribeImages(input)
	if err != nil {
		fmt.Errorf("failed to describe images, %v", err)
	}

	// Print the AMI IDs and their respective tag keys
	for _, image := range result.Images {
		fmt.Printf("Image ID: %s\n", *image.ImageId)
		for _, tag := range image.Tags {
			fmt.Printf("  Tag Key: %s, Tag Value: %s\n", *tag.Key, *tag.Value)
		}
	}
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	var err error

	workdir := ""
	if len(args) > 0 {
		workdir = args[0]
	}

	if opts.NoLimit {
		opts.Limit = -1
	}

	types := []unikraft.ComponentType{}
	if opts.ShowCore {
		types = append(types, unikraft.ComponentTypeCore)
	}
	if opts.ShowArchs {
		types = append(types, unikraft.ComponentTypeArch)
	}
	if opts.ShowPlats {
		types = append(types, unikraft.ComponentTypePlat)
	}
	if opts.ShowLibs {
		types = append(types, unikraft.ComponentTypeLib)
	}
	if opts.ShowApps {
		types = append(types, unikraft.ComponentTypeApp)
	}

	if opts.Update {
		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
			},
			processtree.NewProcessTreeItem(
				"updating",
				"",
				func(ctx context.Context) error {
					return packmanager.G(ctx).Update(ctx)
				},
			),
		)
		if err != nil {
			return err
		}
		if err := treemodel.Start(); err != nil {
			return err
		}
	}

	packages := make(map[string][]pack.Package)

	// List pacakges part of a project
	if f, err := os.Stat(workdir); len(workdir) > 0 && err == nil && f.IsDir() {
		popts := []app.ProjectOption{
			app.WithProjectWorkdir(workdir),
		}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		project, err := app.NewProjectFromOptions(ctx, popts...)
		if err != nil {
			return err
		}

		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo(ctx))
	} else {
		var found []pack.Package

		parallel := !config.G[config.KraftKit](ctx).NoParallel
		norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

		qopts := []packmanager.QueryOption{
			packmanager.WithRemote(opts.Remote),
			packmanager.WithTypes(types...),
		}

		if !opts.ShowArchs && !opts.All {
			if len(opts.Arch) == 0 {
				opts.Arch, err = ukarch.HostArchitecture()
				if err != nil {
					return fmt.Errorf("could not get host architecture: %w", err)
				}
			}

			qopts = append(qopts, packmanager.WithArchitecture(opts.Arch))
		}

		if !opts.ShowPlats && !opts.All {
			if len(opts.Plat) == 0 {
				plat, _, err := mplatform.Detect(ctx)
				if err != nil {
					return fmt.Errorf("could not get host platform: %w", err)
				}

				opts.Plat = plat.String()
			}

			qopts = append(qopts, packmanager.WithPlatform(opts.Plat))
		}

		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
			},
			processtree.NewProcessTreeItem(
				"updating", "",
				func(ctx context.Context) error {
					found, err = packmanager.G(ctx).Catalog(ctx, qopts...)
					if err != nil {
						return err
					}

					return nil
				},
			),
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}

		for _, p := range found {
			format := p.Format().String()
			if _, ok := packages[format]; !ok {
				packages[format] = make([]pack.Package, 0)
			}

			packages[format] = append(packages[format], p)
		}
	}

	if len(packages) == 0 {
		log.G(ctx).Info("no packages found")
		return nil
	}

	for format, packs := range packages {
		// Sort packages by type, name, version, format
		sort.Slice(packs, func(i, j int) bool {
			if packages[format][i].Type() != packages[format][j].Type() {
				return packages[format][i].Type() < packages[format][j].Type()
			}

			if packages[format][i].Name() != packages[format][j].Name() {
				return packages[format][i].Name() < packages[format][j].Name()
			}

			if packages[format][i].Version() != packages[format][j].Version() {
				return packages[format][i].Version() < packages[format][j].Version()
			}

			return packages[format][i].Format() < packages[format][j].Format()
		})
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	for _, packs := range packages {
		if err := pkgutils.PrintPackages(ctx, iostreams.G(ctx).Out, opts.Output, packs...); err != nil {
			return err
		}
	}

	return nil
}
