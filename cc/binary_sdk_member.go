// Copyright 2020 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cc

import (
	"path/filepath"

	"android/soong/android"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"
)

func init() {
	android.RegisterSdkMemberType(ccBinarySdkMemberType)
}

var ccBinarySdkMemberType = &binarySdkMemberType{
	SdkMemberTypeBase: android.SdkMemberTypeBase{
		PropertyName:    "native_binaries",
		HostOsDependent: true,
	},
}

type binarySdkMemberType struct {
	android.SdkMemberTypeBase
}

func (mt *binarySdkMemberType) AddDependencies(ctx android.SdkDependencyContext, dependencyTag blueprint.DependencyTag, names []string) {
	targets := ctx.MultiTargets()
	for _, bin := range names {
		for _, target := range targets {
			variations := target.Variations()
			if ctx.Device() {
				variations = append(variations,
					blueprint.Variation{Mutator: "image", Variation: android.CoreVariation})
			}
			ctx.AddFarVariationDependencies(variations, dependencyTag, bin)
		}
	}
}

func (mt *binarySdkMemberType) IsInstance(module android.Module) bool {
	// Check the module to see if it can be used with this module type.
	if m, ok := module.(*Module); ok {
		for _, allowableMemberType := range m.sdkMemberTypes {
			if allowableMemberType == mt {
				return true
			}
		}
	}

	return false
}

func (mt *binarySdkMemberType) AddPrebuiltModule(ctx android.SdkMemberContext, member android.SdkMember) android.BpModule {
	pbm := ctx.SnapshotBuilder().AddPrebuiltModule(member, "cc_prebuilt_binary")

	ccModule := member.Variants()[0].(*Module)

	if stl := ccModule.stl.Properties.Stl; stl != nil {
		pbm.AddProperty("stl", proptools.String(stl))
	}

	return pbm
}

func (mt *binarySdkMemberType) CreateVariantPropertiesStruct() android.SdkMemberProperties {
	return &nativeBinaryInfoProperties{}
}

const (
	nativeBinaryDir = "bin"
)

// path to the native binary. Relative to <sdk_root>/<api_dir>
func nativeBinaryPathFor(lib nativeBinaryInfoProperties) string {
	return filepath.Join(lib.OsPrefix(), lib.archType,
		nativeBinaryDir, lib.outputFile.Base())
}

// nativeBinaryInfoProperties represents properties of a native binary
//
// The exported (capitalized) fields will be examined and may be changed during common value extraction.
// The unexported fields will be left untouched.
type nativeBinaryInfoProperties struct {
	android.SdkMemberPropertiesBase

	// archType is not exported as if set (to a non default value) it is always arch specific.
	// This is "" for common properties.
	archType string

	// outputFile is not exported as it is always arch specific.
	outputFile android.Path

	// The set of shared libraries
	//
	// This field is exported as its contents may not be arch specific.
	SharedLibs []string

	// The set of system shared libraries
	//
	// This field is exported as its contents may not be arch specific.
	SystemSharedLibs []string

	// Arch specific flags.
	StaticExecutable bool
	Nocrt            bool
}

func (p *nativeBinaryInfoProperties) PopulateFromVariant(ctx android.SdkMemberContext, variant android.Module) {
	ccModule := variant.(*Module)

	p.archType = ccModule.Target().Arch.ArchType.String()
	p.outputFile = getRequiredMemberOutputFile(ctx, ccModule)

	binaryLinker := ccModule.linker.(*binaryDecorator)
	p.StaticExecutable = binaryLinker.static()
	p.Nocrt = Bool(binaryLinker.baseLinker.Properties.Nocrt)

	if ccModule.linker != nil {
		specifiedDeps := specifiedDeps{}
		specifiedDeps = ccModule.linker.linkerSpecifiedDeps(ctx, ccModule, specifiedDeps)

		p.SharedLibs = specifiedDeps.sharedLibs
		p.SystemSharedLibs = specifiedDeps.systemSharedLibs
	}
}

func (p *nativeBinaryInfoProperties) AddToPropertySet(ctx android.SdkMemberContext, propertySet android.BpPropertySet) {
	builder := ctx.SnapshotBuilder()
	if p.outputFile != nil {
		propertySet.AddProperty("srcs", []string{nativeBinaryPathFor(*p)})

		builder.CopyToSnapshot(p.outputFile, nativeBinaryPathFor(*p))
	}

	if len(p.SharedLibs) > 0 {
		propertySet.AddPropertyWithTag("shared_libs", p.SharedLibs, builder.SdkMemberReferencePropertyTag(false))
	}

	// SystemSharedLibs needs to be propagated if it's a list, even if it's empty,
	// so check for non-nil instead of nonzero length.
	if p.SystemSharedLibs != nil {
		propertySet.AddPropertyWithTag("system_shared_libs", p.SystemSharedLibs, builder.SdkMemberReferencePropertyTag(false))
	}

	if p.StaticExecutable {
		propertySet.AddProperty("static_executable", p.StaticExecutable)
	}
	if p.Nocrt {
		propertySet.AddProperty("nocrt", p.Nocrt)
	}
}
