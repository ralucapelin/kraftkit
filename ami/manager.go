// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ami

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
	fmt.Println(exportImageToS3("ami-0afd61ac4b95bafea", "kraftkit"))
	//checkExportTaskStatus("export-ami-0f58bb73ba9ec739b")
	//fmt.Println("Created role")
	//DeregisterImageByName("named-ami")
	return nil
}

type MyString string

// Implement the String method for MyString type
func (s MyString) String() string {
	return string(s)
}

// parseOSAndArch parses a string in the format "OS: <os>, Architecture: <arch>"
// and returns the os and arch as separate strings.
func parseOSAndArch(input string) (string, string, error) {
	// Split the input string by comma
	parts := strings.Split(input, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid input format")
	}

	// Extract the OS part
	osPart := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(osPart, "OS: ") {
		return "", "", fmt.Errorf("invalid OS format")
	}
	os := strings.TrimPrefix(osPart, "OS: ")

	// Extract the Architecture part
	archPart := strings.TrimSpace(parts[1])
	if !strings.HasPrefix(archPart, "Architecture: ") {
		return "", "", fmt.Errorf("invalid Architecture format")
	}
	arch := strings.TrimPrefix(archPart, "Architecture: ")

	return os, arch, nil
}

func getImage(ref string) []MyString {
	img, _ := name.ParseReference("index.unikraft.io/" + ref)
	optAuth := remote.WithAuthFromKeychain(authn.DefaultKeychain)
	desc, err := remote.Get(img, optAuth)
	if err != nil {
		fmt.Errorf("obtaining remote image descriptor: %w", err)
		return nil
	}

	var platformOptions []MyString
	// Check if the descriptor references an index (manifest list) or a single image
	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex:
		// Fetch the image index
		index, err := desc.ImageIndex()
		if err != nil {
			fmt.Errorf("Failed to get image index: %v", err)
		}

		// Get the index manifest
		indexManifest, err := index.IndexManifest()
		if err != nil {
			fmt.Errorf("Failed to get index manifest: %v", err)
		}

		// Iterate over the manifests and print platform details
		for _, manifest := range indexManifest.Manifests {
			if manifest.Platform != nil {
				platformOptions = append(platformOptions, MyString(fmt.Sprintf("OS: %s, Architecture: %s\n", manifest.Platform.OS, manifest.Platform.Architecture)))
				fmt.Printf("OS: %s, Architecture: %s\n", manifest.Platform.OS, manifest.Platform.Architecture)
			} else {
				fmt.Println("Platform information not available for this manifest")
			}
		}
		return platformOptions

	case ocispec.MediaTypeImageManifest:
		// Single manifest, handle it accordingly
		fmt.Println("Single manifest found, no multi-platform details.")
		img, err := desc.Image()
		if err != nil {
			fmt.Errorf("Failed to get image: %v", err)
		}
		cfg, err := img.ConfigFile()
		if err != nil {
			fmt.Errorf("Failed to get image config file: %v", err)
		}
		platform := cfg.OS + "/" + cfg.Architecture
		if cfg.Variant != "" {
			platform += "/" + cfg.Variant
		}
		fmt.Printf("OS: %s, Architecture: %s, Variant: %s\n", cfg.OS, cfg.Architecture, cfg.Variant)
		return platformOptions

	default:
		fmt.Printf("Unsupported media type: %s\n", desc.MediaType)
		return platformOptions
	}
}

// Pack implements packmanager.PackageManager
func (manager *amiManager) Pack(ctx context.Context, entity component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	options := []MyString{"I want to configure IAM policies by myself", "I want IAM policies to be configured automatically", "Exit"}
	choice, err := selection.Select[MyString]("Configuring necessary IAM policies: how would you like to proceed?", options...)

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

	var queueURLs = CreateQueues()
	fmt.Println(queueURLs)
	//time.Sleep(5 * time.Second)
	popts := packmanager.NewPackOptions()
	for _, opt := range opts {
		opt(popts)
	}
	name := popts.Name()
	fmt.Println(name)

	options = getImage(name)
	choice, err = selection.Select[MyString]("Choose the platform for your image:", options...)

	startTime := time.Now()
	os, arch, err := parseOSAndArch(choice.String())

	value := "my-ami"
	instanceProfileName := "kraftkit-role"

	var result, errEC2 = MakeInstance(&name, &value, instanceProfileName)
	duration := time.Since(startTime)
	fmt.Printf("The function took %s to complete.\n", duration)
	fmt.Println(result)
	if errEC2 != nil {
		fmt.Println("Error when launching EC2 instance")
	}
	fmt.Println("successfully created instance")
	time.Sleep(10 * time.Second)
	//	fmt.Println("sending build order...")
	SendBuildOrder(name, os, arch)
	fmt.Println("building AMI...")
	time.Sleep(40 * time.Second)
	amiID, _ := ReceiveResult()

	fmt.Printf("AMI ID: %s\n", amiID)

	time.Sleep(5000 * time.Millisecond)
	DeleteInstanceProfileAndRole(instanceProfileName)
	DeleteQueues(queueURLs)

	var instanceID = *result.Instances[0].InstanceId

	TerminateInstance(instanceID)
	duration = time.Since(startTime)
	fmt.Printf("The function took %s to complete.\n", duration)
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
