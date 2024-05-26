// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ami

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/go-digest"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

const ConfigFilename = "config.json"

// amiPackage works by referencing a specific manifest which represents the
// "package" as well as the index that manifest should be part of.  When
// when internally referencing the packaged entity, this is the manifest and its
// representation is presented via the index.
type amiPackage struct {
	handle handler.Handler
	ref    name.Reference
	auths  map[string]config.AuthConfig

	// Embedded attributes which represent target.Target
	arch      arch.Architecture
	plat      plat.Platform
	kconfig   kconfig.KeyValueMap
	kernel    string
	kernelDbg string
	initrd    initrd.Initrd
	command   []string

	original *amiPackage
}

var (
	_ pack.Package  = (*amiPackage)(nil)
	_ target.Target = (*amiPackage)(nil)
)

// NewPackageFromTarget generates an OCI implementation of the pack.Package
// construct based on an input Application and options.
func NewPackageFromTarget(ctx context.Context, targ target.Target, opts ...packmanager.PackOption) (pack.Package, error) {

	return nil, nil
}

// NewPackageFromOCIManifestDigest is a constructor method which
// instantiates a package based on the OCI format based on a provided OCI
// Image manifest digest.
func NewPackageFromOCIManifestDigest(ctx context.Context, handle handler.Handler, ref string, auths map[string]config.AuthConfig, dgst digest.Digest) (pack.Package, error) {
	amipack := amiPackage{
		handle: handle,
		auths:  auths,
	}

	return &amipack, nil
}

// Type implements unikraft.Nameable
func (amipack *amiPackage) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements unikraft.Nameable
func (amipack *amiPackage) Name() string {
	return amipack.ref.Context().Name()
}

// ID implements pack.Package
func (amipack *amiPackage) ID() string {
	return ""
}

// Name implements fmt.Stringer
func (amipack *amiPackage) String() string {
	return fmt.Sprintf("%s (%s/%s)", amipack.imageRef(), amipack.Platform().Name(), amipack.Architecture().Name())
}

// Version implements unikraft.Nameable
func (amipack *amiPackage) Version() string {
	return amipack.ref.Identifier()
}

// imageRef returns the OCI-standard image name in the format `name:tag`
func (amipack *amiPackage) imageRef() string {
	if strings.HasPrefix(amipack.Version(), "sha256:") {
		return fmt.Sprintf("%s@%s", amipack.Name(), amipack.Version())
	}
	return fmt.Sprintf("%s:%s", amipack.Name(), amipack.Version())
}

// Metadata implements pack.Package
func (amipack *amiPackage) Metadata() interface{} {
	return nil
}

// Columns implements pack.Package
func (amipack *amiPackage) Columns() []tableprinter.Column {
	size := "n/a"

	return []tableprinter.Column{
		{Name: "plat", Value: fmt.Sprintf("%s/%s", amipack.Platform().Name(), amipack.Architecture().Name())},
		{Name: "size", Value: size},
	}
}

// Push implements pack.Package
func (amipack *amiPackage) Push(ctx context.Context, opts ...pack.PushOption) error {
	return nil
}

// Unpack implements pack.Package
func (amipack *amiPackage) Unpack(ctx context.Context, dir string) error {
	return nil
}

// Pull implements pack.Package
func (amipack *amiPackage) Pull(ctx context.Context, opts ...pack.PullOption) error {
	return nil
}

// PulledAt implements pack.Package
func (amipack *amiPackage) PulledAt(ctx context.Context) (bool, time.Time, error) {
	return false, time.Time{}, nil
}

// Delete implements pack.Package.
func (amipack *amiPackage) Delete(ctx context.Context) error {
	return nil
}

// Save implements pack.Package
func (amipack *amiPackage) Save(ctx context.Context) error {
	return nil
}

// Pull implements pack.Package
func (amipack *amiPackage) Format() pack.PackageFormat {
	return AMIFormat
}

// Source implements unikraft.target.Target
func (amipack *amiPackage) Source() string {
	return ""
}

// Path implements unikraft.target.Target
func (amipack *amiPackage) Path() string {
	return ""
}

// KConfigTree implements unikraft.target.Target
func (amipack *amiPackage) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not implemented: oci.amiPackage.KConfigTree")
}

// KConfig implements unikraft.target.Target
func (amipack *amiPackage) KConfig() kconfig.KeyValueMap {
	return amipack.kconfig
}

// PrintInfo implements unikraft.target.Target
func (amipack *amiPackage) PrintInfo(context.Context) string {
	return "not implemented: oci.amiPackage.PrintInfo"
}

// Architecture implements unikraft.target.Target
func (amipack *amiPackage) Architecture() arch.Architecture {
	return amipack.arch
}

// Platform implements unikraft.target.Target
func (amipack *amiPackage) Platform() plat.Platform {
	return amipack.plat
}

// Kernel implements unikraft.target.Target
func (amipack *amiPackage) Kernel() string {
	return amipack.kernel
}

// KernelDbg implements unikraft.target.Target
func (amipack *amiPackage) KernelDbg() string {
	return amipack.kernelDbg
}

// Initrd implements unikraft.target.Target
func (amipack *amiPackage) Initrd() initrd.Initrd {
	return amipack.initrd
}

// Command implements unikraft.target.Target
func (amipack *amiPackage) Command() []string {
	return amipack.command
}

// ConfigFilename implements unikraft.target.Target
func (amipack *amiPackage) ConfigFilename() string {
	return ""
}

// MarshalYAML implements unikraft.target.Target (yaml.Marshaler)
func (amipack *amiPackage) MarshalYAML() (interface{}, error) {
	if amipack == nil {
		return nil, nil
	}

	return map[string]interface{}{
		"architecture": amipack.arch.Name(),
		"platform":     amipack.plat.Name(),
	}, nil
}
