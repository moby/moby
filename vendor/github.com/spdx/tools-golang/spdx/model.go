// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

// Package spdx contains references to the latest spdx version
package spdx

import (
	"github.com/spdx/tools-golang/spdx/v2/common"
	latest "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

const (
	Version     = latest.Version
	DataLicense = latest.DataLicense
)

type (
	Annotation               = latest.Annotation
	ArtifactOfProject        = latest.ArtifactOfProject
	CreationInfo             = latest.CreationInfo
	Document                 = latest.Document
	ExternalDocumentRef      = latest.ExternalDocumentRef
	File                     = latest.File
	OtherLicense             = latest.OtherLicense
	Package                  = latest.Package
	PackageExternalReference = latest.PackageExternalReference
	Relationship             = latest.Relationship
	Review                   = latest.Review
	Snippet                  = latest.Snippet
)

type (
	Annotator               = common.Annotator
	Checksum                = common.Checksum
	ChecksumAlgorithm       = common.ChecksumAlgorithm
	Creator                 = common.Creator
	DocElementID            = common.DocElementID
	ElementID               = common.ElementID
	Originator              = common.Originator
	PackageVerificationCode = common.PackageVerificationCode
	SnippetRange            = common.SnippetRange
	SnippetRangePointer     = common.SnippetRangePointer
	Supplier                = common.Supplier
)

const (
	SHA224      = common.SHA224
	SHA1        = common.SHA1
	SHA256      = common.SHA256
	SHA384      = common.SHA384
	SHA512      = common.SHA512
	MD2         = common.MD2
	MD4         = common.MD4
	MD5         = common.MD5
	MD6         = common.MD6
	SHA3_256    = common.SHA3_256
	SHA3_384    = common.SHA3_384
	SHA3_512    = common.SHA3_512
	BLAKE2b_256 = common.BLAKE2b_256
	BLAKE2b_384 = common.BLAKE2b_384
	BLAKE2b_512 = common.BLAKE2b_512
	BLAKE3      = common.BLAKE3
	ADLER32     = common.ADLER32
)

const (
	// F.2 Security types
	CategorySecurity  = common.CategorySecurity
	SecurityCPE23Type = common.TypeSecurityCPE23Type
	SecurityCPE22Type = common.TypeSecurityCPE22Type
	SecurityAdvisory  = common.TypeSecurityAdvisory
	SecurityFix       = common.TypeSecurityFix
	SecurityUrl       = common.TypeSecurityUrl
	SecuritySwid      = common.TypeSecuritySwid

	// F.3 Package-Manager types
	CategoryPackageManager     = common.CategoryPackageManager
	PackageManagerMavenCentral = common.TypePackageManagerMavenCentral
	PackageManagerNpm          = common.TypePackageManagerNpm
	PackageManagerNuGet        = common.TypePackageManagerNuGet
	PackageManagerBower        = common.TypePackageManagerBower
	PackageManagerPURL         = common.TypePackageManagerPURL

	// F.4 Persistent-Id types
	CategoryPersistentId   = common.CategoryPersistentId
	TypePersistentIdSwh    = common.TypePersistentIdSwh
	TypePersistentIdGitoid = common.TypePersistentIdGitoid

	// 11.1 Relationship field types
	RelationshipDescribes                 = common.TypeRelationshipDescribe
	RelationshipDescribedBy               = common.TypeRelationshipDescribeBy
	RelationshipContains                  = common.TypeRelationshipContains
	RelationshipContainedBy               = common.TypeRelationshipContainedBy
	RelationshipDependsOn                 = common.TypeRelationshipDependsOn
	RelationshipDependencyOf              = common.TypeRelationshipDependencyOf
	RelationshipBuildDependencyOf         = common.TypeRelationshipBuildDependencyOf
	RelationshipDevDependencyOf           = common.TypeRelationshipDevDependencyOf
	RelationshipOptionalDependencyOf      = common.TypeRelationshipOptionalDependencyOf
	RelationshipProvidedDependencyOf      = common.TypeRelationshipProvidedDependencyOf
	RelationshipTestDependencyOf          = common.TypeRelationshipTestDependencyOf
	RelationshipRuntimeDependencyOf       = common.TypeRelationshipRuntimeDependencyOf
	RelationshipExampleOf                 = common.TypeRelationshipExampleOf
	RelationshipGenerates                 = common.TypeRelationshipGenerates
	RelationshipGeneratedFrom             = common.TypeRelationshipGeneratedFrom
	RelationshipAncestorOf                = common.TypeRelationshipAncestorOf
	RelationshipDescendantOf              = common.TypeRelationshipDescendantOf
	RelationshipVariantOf                 = common.TypeRelationshipVariantOf
	RelationshipDistributionArtifact      = common.TypeRelationshipDistributionArtifact
	RelationshipPatchFor                  = common.TypeRelationshipPatchFor
	RelationshipPatchApplied              = common.TypeRelationshipPatchApplied
	RelationshipCopyOf                    = common.TypeRelationshipCopyOf
	RelationshipFileAdded                 = common.TypeRelationshipFileAdded
	RelationshipFileDeleted               = common.TypeRelationshipFileDeleted
	RelationshipFileModified              = common.TypeRelationshipFileModified
	RelationshipExpandedFromArchive       = common.TypeRelationshipExpandedFromArchive
	RelationshipDynamicLink               = common.TypeRelationshipDynamicLink
	RelationshipStaticLink                = common.TypeRelationshipStaticLink
	RelationshipDataFileOf                = common.TypeRelationshipDataFileOf
	RelationshipTestCaseOf                = common.TypeRelationshipTestCaseOf
	RelationshipBuildToolOf               = common.TypeRelationshipBuildToolOf
	RelationshipDevToolOf                 = common.TypeRelationshipDevToolOf
	RelationshipTestOf                    = common.TypeRelationshipTestOf
	RelationshipTestToolOf                = common.TypeRelationshipTestToolOf
	RelationshipDocumentationOf           = common.TypeRelationshipDocumentationOf
	RelationshipOptionalComponentOf       = common.TypeRelationshipOptionalComponentOf
	RelationshipMetafileOf                = common.TypeRelationshipMetafileOf
	RelationshipPackageOf                 = common.TypeRelationshipPackageOf
	RelationshipAmends                    = common.TypeRelationshipAmends
	RelationshipPrerequisiteFor           = common.TypeRelationshipPrerequisiteFor
	RelationshipHasPrerequisite           = common.TypeRelationshipHasPrerequisite
	RelationshipRequirementDescriptionFor = common.TypeRelationshipRequirementDescriptionFor
	RelationshipSpecificationFor          = common.TypeRelationshipSpecificationFor
	RelationshipOther                     = common.TypeRelationshipOther
)
