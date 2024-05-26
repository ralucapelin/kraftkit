// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ami

import (
	"context"
	"fmt"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
	ociutils "kraftkit.sh/oci/utils"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/component"
)

type amiManager struct {
	registries []string
	auths      map[string]config.AuthConfig
	handle     func(ctx context.Context) (context.Context, handler.Handler, error)
}

const AMIFormat pack.PackageFormat = "ami"

// NewAMIManager instantiates a new package manager based on AMIs.
func NewAMIManager(ctx context.Context, opts ...any) (packmanager.PackageManager, error) {
	manager := amiManager{}

	for _, mopt := range opts {
		opt, ok := mopt.(AMIManagerOption)
		if !ok {
			return nil, fmt.Errorf("cannot cast OCI Manager option")
		}

		if err := opt(ctx, &manager); err != nil {
			return nil, err
		}
	}

	return &manager, nil
}

// Update implements packmanager.PackageManager
func (manager *amiManager) Update(ctx context.Context) error {
	fmt.Println("HERE")
	//createS3Bucket("kraftkit")
	//CreateVMImportRole("kraftkit")
	//time.Sleep(10 * time.Second)
	//fmt.Println(exportImageToS3("ami-07b96fdece051edd9", "kraftkit"))
	checkExportTaskStatus("export-ami-0f58bb73ba9ec739b")
	//fmt.Println("Created role")
	return nil
}

type MyString string

// Implement the String method for MyString type
func (s MyString) String() string {
	return string(s)
}

// Pack implements packmanager.PackageManager
func (manager *amiManager) Pack(ctx context.Context, entity component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	options := []MyString{"I want to configure IAM policies by myself", "I want IAM policies to be configured automatically", "Exit"}
	choice, err := selection.Select[MyString]("multiple runnable contexts discovered: how would you like to proceed?", options...)

	if choice.String() == (&options[0]).String() {
		log.G(ctx).Trace("I want to configure IAM policies")
	}
	if choice.String() == (&options[1]).String() {
		log.G(ctx).Trace("I want IAM policies to be configured")
		AddPolicies()
	}
	if choice.String() == (&options[2]).String() {
		log.G(ctx).Trace("exit")
		return []pack.Package{}, err
	}

	// name := "test"
	// value := "test"

	// var result, errEC2 = MakeInstance(&name, &value)
	// fmt.Println(result)
	// if errEC2 != nil {
	// 	fmt.Println("Error when launching EC2 instance")
	// }

	var queueURLs = CreateQueues()
	fmt.Println(queueURLs)
	//time.Sleep(1000 * time.Millisecond)

	//DeleteQueues(queueURLs)

	//var instanceID = *result.Instances[0].InstanceId

	//TerminateInstance(instanceID)
	return []pack.Package{}, err
}

// Unpack implements packmanager.PackageManager
func (manager *amiManager) Unpack(ctx context.Context, entity pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented: oci.manager.Unpack")
}

// processV1IndexManifests is an internal utility method which is able to
// iterate over the supplied slice of ocispec.Descriptors which represent a
// Manifest from an Index.  Based on the provided criterium from the query,
// identify the Descriptor that is compatible and instantiate a pack.Package
// structure from it.
func processV1IndexManifests(ctx context.Context, handle handler.Handler, fullref string, query *packmanager.Query, manifests []ocispec.Descriptor) map[string]pack.Package {
	packs := make(map[string]pack.Package)
	var wg sync.WaitGroup
	wg.Add(len(manifests))
	var mu sync.RWMutex

	for _, descriptor := range manifests {
		go func(descriptor ocispec.Descriptor) {
			defer wg.Done()

			if query != nil && query.Platform() != "" && query.Platform() != descriptor.Platform.OS {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					WithField("want", query.Platform()).
					WithField("got", descriptor.Platform.OS).
					Trace("skipping manifest: platform does not match query")
				return
			}

			if query != nil && query.Architecture() != "" && query.Architecture() != descriptor.Platform.Architecture {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					WithField("want", query.Architecture()).
					WithField("got", descriptor.Platform.Architecture).
					Trace("skipping manifest: architecture does not match query")
				return
			}

			if query != nil && len(query.KConfig()) > 0 {
				// If the list of requested features is greater than the list of
				// available features, there will be no way for the two to match.  We
				// are searching for a subset of query.KConfig() from
				// m.Platform.OSFeatures to match.
				if len(query.KConfig()) > len(descriptor.Platform.OSFeatures) {
					log.G(ctx).
						WithField("ref", fullref).
						WithField("digest", descriptor.Digest.String()).
						Trace("skipping descriptor: query contains more features than available")
					return
				}

				available := set.NewStringSet(descriptor.Platform.OSFeatures...)

				// Iterate through the query's requested set of features and skip only
				// if the descriptor does not contain the requested KConfig feature.
				for _, a := range query.KConfig() {
					if !available.Contains(a) {
						log.G(ctx).
							WithField("ref", fullref).
							WithField("digest", descriptor.Digest.String()).
							WithField("feature", a).
							Trace("skipping manifest: missing feature")
						return
					}
				}
			}

			var auths map[string]config.AuthConfig
			if query != nil {
				auths = query.Auths()
			}

			log.G(ctx).
				WithField("ref", fullref).
				WithField("digest", descriptor.Digest.String()).
				Trace("found")

			// If we have made it this far, the query has been successfully
			// satisfied by this particular manifest and we can generate a package
			// from it.
			pack, err := NewPackageFromOCIManifestDigest(ctx,
				handle,
				fullref,
				auths,
				descriptor.Digest,
			)
			if err != nil {
				log.G(ctx).
					WithField("ref", fullref).
					WithField("digest", descriptor.Digest.String()).
					Tracef("skipping manifest: could not instantiate package from manifest digest: %s", err.Error())
				return
			}

			checksum, err := ociutils.PlatformChecksum(pack.String(), descriptor.Platform)
			if err != nil {
				log.G(ctx).
					WithField("ref", fullref).
					Debugf("could not calculate platform digest for '%s': %s", descriptor.Digest.String(), err)
				return
			}

			mu.Lock()
			packs[checksum] = pack
			mu.Unlock()
		}(descriptor)
	}

	wg.Wait()

	return packs
}

// Catalog implements packmanager.PackageManager
func (manager *amiManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	return nil, nil
}

// SetSources implements packmanager.PackageManager
func (manager *amiManager) SetSources(_ context.Context, sources ...string) error {
	manager.registries = sources
	return nil
}

// AddSource implements packmanager.PackageManager
func (manager *amiManager) AddSource(ctx context.Context, source string) error {
	return nil
}

// Delete implements packmanager.PackageManager.
func (manager *amiManager) Delete(ctx context.Context, qopts ...packmanager.QueryOption) error {
	query := packmanager.NewQuery(qopts...)

	fmt.Println(query.Name())
	var snapshots = DeregisterImageByName(query.Name())
	fmt.Println("Deregistered ami successfully")
	DeleteSnapshots(snapshots)
	fmt.Println("Deleted snapshots  successfully")

	return nil
}

// RemoveSource implements packmanager.PackageManager
func (manager *amiManager) RemoveSource(ctx context.Context, source string) error {
	return nil
}

// IsCompatible implements packmanager.PackageManager
func (manager *amiManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {

	if isValidAMI(source) {
		return manager, true, nil
	}
	return nil, false, nil
}

// From implements packmanager.PackageManager
func (manager *amiManager) From(pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("not possible: ami.manager.From")
}

// Format implements packmanager.PackageManager
func (manager *amiManager) Format() pack.PackageFormat {
	return AMIFormat
}
