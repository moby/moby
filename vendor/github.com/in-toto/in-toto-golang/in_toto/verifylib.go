/*
Package in_toto implements types and routines to verify a software supply chain
according to the in-toto specification.
See https://github.com/in-toto/docs/blob/master/in-toto-spec.md
*/
package in_toto

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// ErrInspectionRunDirIsSymlink gets thrown if the runDir is a symlink
var ErrInspectionRunDirIsSymlink = errors.New("runDir is a symlink. This is a security risk")

var ErrNotLayout = errors.New("verification workflow passed a non-layout")

/*
RunInspections iteratively executes the command in the Run field of all
inspections of the passed layout, creating unsigned link metadata that records
all files found in the current working directory as materials (before command
execution) and products (after command execution).  A map with inspection names
as keys and Metablocks containing the generated link metadata as values is
returned.  The format is:

	{
		<inspection name> : Metablock,
		<inspection name> : Metablock,
		...
	}

If executing the inspection command fails, or if the executed command has a
non-zero exit code, the first return value is an empty Metablock map and the
second return value is the error.
*/
func RunInspections(layout Layout, runDir string, lineNormalization bool, useDSSE bool) (map[string]Metadata, error) {
	inspectionMetadata := make(map[string]Metadata)

	for _, inspection := range layout.Inspect {

		paths := []string{"."}
		if runDir != "" {
			paths = []string{runDir}
		}

		linkEnv, err := InTotoRun(inspection.Name, runDir, paths, paths,
			inspection.Run, Key{}, []string{"sha256"}, nil, nil, lineNormalization, false, useDSSE)

		if err != nil {
			return nil, err
		}

		retVal := linkEnv.GetPayload().(Link).ByProducts["return-value"]
		if retVal != float64(0) {
			return nil, fmt.Errorf("inspection command '%s' of inspection '%s'"+
				" returned a non-zero value: %d", inspection.Run, inspection.Name,
				retVal)
		}

		// Dump inspection link to cwd using the short link name format
		linkName := fmt.Sprintf(LinkNameFormatShort, inspection.Name)
		if err := linkEnv.Dump(linkName); err != nil {
			fmt.Printf("JSON serialization or writing failed: %s", err)
		}

		inspectionMetadata[inspection.Name] = linkEnv
	}
	return inspectionMetadata, nil
}

// verifyMatchRule is a helper function to process artifact rules of
// type MATCH. See VerifyArtifacts for more details.
func verifyMatchRule(ruleData map[string]string,
	srcArtifacts map[string]interface{}, srcArtifactQueue Set,
	itemsMetadata map[string]Metadata) Set {
	consumed := NewSet()
	// Get destination link metadata
	dstLinkEnv, exists := itemsMetadata[ruleData["dstName"]]
	if !exists {
		// Destination link does not exist, rule can't consume any
		// artifacts
		return consumed
	}

	// Get artifacts from destination link metadata
	var dstArtifacts map[string]interface{}
	switch ruleData["dstType"] {
	case "materials":
		dstArtifacts = dstLinkEnv.GetPayload().(Link).Materials
	case "products":
		dstArtifacts = dstLinkEnv.GetPayload().(Link).Products
	}

	// cleanup paths in pattern and artifact maps
	if ruleData["pattern"] != "" {
		ruleData["pattern"] = path.Clean(ruleData["pattern"])
	}
	for k := range srcArtifacts {
		if path.Clean(k) != k {
			srcArtifacts[path.Clean(k)] = srcArtifacts[k]
			delete(srcArtifacts, k)
		}
	}
	for k := range dstArtifacts {
		if path.Clean(k) != k {
			dstArtifacts[path.Clean(k)] = dstArtifacts[k]
			delete(dstArtifacts, k)
		}
	}

	// Normalize optional source and destination prefixes, i.e. if
	// there is a prefix, then add a trailing slash if not there yet
	for _, prefix := range []string{"srcPrefix", "dstPrefix"} {
		if ruleData[prefix] != "" {
			ruleData[prefix] = path.Clean(ruleData[prefix])
			if !strings.HasSuffix(ruleData[prefix], "/") {
				ruleData[prefix] += "/"
			}
		}
	}
	// Iterate over queue and mark consumed artifacts
	for srcPath := range srcArtifactQueue {
		// Remove optional source prefix from source artifact path
		// Noop if prefix is empty, or artifact does not have it
		srcBasePath := strings.TrimPrefix(srcPath, ruleData["srcPrefix"])

		// Ignore artifacts not matched by rule pattern
		matched, err := match(ruleData["pattern"], srcBasePath)
		if err != nil || !matched {
			continue
		}

		// Construct corresponding destination artifact path, i.e.
		// an optional destination prefix plus the source base path
		dstPath := path.Clean(path.Join(ruleData["dstPrefix"], srcBasePath))

		// Try to find the corresponding destination artifact
		dstArtifact, exists := dstArtifacts[dstPath]
		// Ignore artifacts without corresponding destination artifact
		if !exists {
			continue
		}

		// Ignore artifact pairs with no matching hashes
		if !reflect.DeepEqual(srcArtifacts[srcPath], dstArtifact) {
			continue
		}

		// Only if a source and destination artifact pair was found and
		// their hashes are equal, will we mark the source artifact as
		// successfully consumed, i.e. it will be removed from the queue
		consumed.Add(srcPath)
	}
	return consumed
}

/*
VerifyArtifacts iteratively applies the material and product rules of the
passed items (step or inspection) to enforce and authorize artifacts (materials
or products) reported by the corresponding link and to guarantee that
artifacts are linked together across links.  In the beginning all artifacts are
placed in a queue according to their type.  If an artifact gets consumed by a
rule it is removed from the queue.  An artifact can only be consumed once in
the course of processing the set of rules in ExpectedMaterials or
ExpectedProducts.

Rules of type MATCH, ALLOW, CREATE, DELETE, MODIFY and DISALLOW are supported.

All rules except for DISALLOW consume queued artifacts on success, and
leave the queue unchanged on failure.  Hence, it is left to a terminal
DISALLOW rule to fail overall verification, if artifacts are left in the queue
that should have been consumed by preceding rules.
*/
func VerifyArtifacts(items []interface{},
	itemsMetadata map[string]Metadata) error {
	// Verify artifact rules for each item in the layout
	for _, itemI := range items {
		// The layout item (interface) must be a Link or an Inspection we are only
		// interested in the name and the expected materials and products
		var itemName string
		var expectedMaterials [][]string
		var expectedProducts [][]string

		switch item := itemI.(type) {
		case Step:
			itemName = item.Name
			expectedMaterials = item.ExpectedMaterials
			expectedProducts = item.ExpectedProducts

		case Inspection:
			itemName = item.Name
			expectedMaterials = item.ExpectedMaterials
			expectedProducts = item.ExpectedProducts

		default: // Something wrong
			return fmt.Errorf("VerifyArtifacts received an item of invalid type,"+
				" elements of passed slice 'items' must be one of 'Step' or"+
				" 'Inspection', got: '%s'", reflect.TypeOf(item))
		}

		// Use the item's name to extract the corresponding link
		srcLinkEnv, exists := itemsMetadata[itemName]
		if !exists {
			return fmt.Errorf("VerifyArtifacts could not find metadata"+
				" for item '%s', got: '%s'", itemName, itemsMetadata)
		}

		// Create shortcuts to materials and products (including hashes) reported
		// by the item's link, required to verify "match" rules
		materials := srcLinkEnv.GetPayload().(Link).Materials
		products := srcLinkEnv.GetPayload().(Link).Products

		// All other rules only require the material or product paths (without
		// hashes). We extract them from the corresponding maps and store them as
		// sets for convenience in further processing
		materialPaths := NewSet()
		for _, p := range InterfaceKeyStrings(materials) {
			materialPaths.Add(path.Clean(p))
		}
		productPaths := NewSet()
		for _, p := range InterfaceKeyStrings(products) {
			productPaths.Add(path.Clean(p))
		}

		// For `create`, `delete` and `modify` rules we prepare sets of artifacts
		// (without hashes) that were created, deleted or modified in the current
		// step or inspection
		created := productPaths.Difference(materialPaths)
		deleted := materialPaths.Difference(productPaths)
		remained := materialPaths.Intersection(productPaths)
		modified := NewSet()
		for name := range remained {
			if !reflect.DeepEqual(materials[name], products[name]) {
				modified.Add(name)
			}
		}

		// For each item we have to run rule verification, once per artifact type.
		// Here we prepare the corresponding data for each round.
		verificationDataList := []map[string]interface{}{
			{
				"srcType":       "materials",
				"rules":         expectedMaterials,
				"artifacts":     materials,
				"artifactPaths": materialPaths,
			},
			{
				"srcType":       "products",
				"rules":         expectedProducts,
				"artifacts":     products,
				"artifactPaths": productPaths,
			},
		}
		// TODO: Add logging library (see in-toto/in-toto-golang#4)
		// fmt.Printf("Verifying %s '%s' ", reflect.TypeOf(itemI), itemName)

		// Process all material rules using the corresponding materials and all
		// product rules using the corresponding products
		for _, verificationData := range verificationDataList {
			// TODO: Add logging library (see in-toto/in-toto-golang#4)
			// fmt.Printf("%s...\n", verificationData["srcType"])

			rules := verificationData["rules"].([][]string)
			artifacts := verificationData["artifacts"].(map[string]interface{})

			// Use artifacts (without hashes) as base queue. Each rule only operates
			// on artifacts in that queue.  If a rule consumes an artifact (i.e. can
			// be applied successfully), the artifact is removed from the queue. By
			// applying a DISALLOW rule eventually, verification may return an error,
			// if the rule matches any artifacts in the queue that should have been
			// consumed earlier.
			queue := verificationData["artifactPaths"].(Set)

			// TODO: Add logging library (see in-toto/in-toto-golang#4)
			// fmt.Printf("Initial state\nMaterials: %s\nProducts: %s\nQueue: %s\n\n",
			// 	materialPaths.Slice(), productPaths.Slice(), queue.Slice())

			// Verify rules sequentially
			for _, rule := range rules {
				// Parse rule and error out if it is malformed
				// NOTE: the rule format should have been validated before
				ruleData, err := UnpackRule(rule)
				if err != nil {
					return err
				}

				// Apply rule pattern to filter queued artifacts that are up for rule
				// specific consumption
				filtered := queue.Filter(path.Clean(ruleData["pattern"]))

				var consumed Set
				switch ruleData["type"] {
				case "match":
					// Note: here we need to perform more elaborate filtering
					consumed = verifyMatchRule(ruleData, artifacts, queue, itemsMetadata)

				case "allow":
					// Consumes all filtered artifacts
					consumed = filtered

				case "create":
					// Consumes filtered artifacts that were created
					consumed = filtered.Intersection(created)

				case "delete":
					// Consumes filtered artifacts that were deleted
					consumed = filtered.Intersection(deleted)

				case "modify":
					// Consumes filtered artifacts that were modified
					consumed = filtered.Intersection(modified)

				case "disallow":
					// Does not consume but errors out if artifacts were filtered
					if len(filtered) > 0 {
						return fmt.Errorf("artifact verification failed for %s '%s',"+
							" %s %s disallowed by rule %s",
							reflect.TypeOf(itemI).Name(), itemName,
							verificationData["srcType"], filtered.Slice(), rule)
					}
				case "require":
					// REQUIRE is somewhat of a weird animal that does not use
					// patterns bur rather single filenames (for now).
					if !queue.Has(ruleData["pattern"]) {
						return fmt.Errorf("artifact verification failed for %s in REQUIRE '%s',"+
							" because %s is not in %s", verificationData["srcType"],
							ruleData["pattern"], ruleData["pattern"], queue.Slice())
					}
				}
				// Update queue by removing consumed artifacts
				queue = queue.Difference(consumed)
				// TODO: Add logging library (see in-toto/in-toto-golang#4)
				// fmt.Printf("Rule: %s\nQueue: %s\n\n", rule, queue.Slice())
			}
		}
	}
	return nil
}

/*
ReduceStepsMetadata merges for each step of the passed Layout all the passed
per-functionary links into a single link, asserting that the reported Materials
and Products are equal across links for a given step.  This function may be
used at a time during the overall verification, where link threshold's have
been verified and subsequent verification only needs one exemplary link per
step.  The function returns a map with one Metablock (link) per step:

	{
		<step name> : Metablock,
		<step name> : Metablock,
		...
	}

If links corresponding to the same step report different Materials or different
Products, the first return value is an empty Metablock map and the second
return value is the error.
*/
func ReduceStepsMetadata(layout Layout,
	stepsMetadata map[string]map[string]Metadata) (map[string]Metadata,
	error) {
	stepsMetadataReduced := make(map[string]Metadata)

	for _, step := range layout.Steps {
		linksPerStep, ok := stepsMetadata[step.Name]
		// We should never get here, layout verification must fail earlier
		if !ok || len(linksPerStep) < 1 {
			panic("Could not reduce metadata for step '" + step.Name +
				"', no link metadata found.")
		}

		// Get the first link (could be any link) for the current step, which will
		// serve as reference link for below comparisons
		var referenceKeyID string
		var referenceLinkEnv Metadata
		for keyID, linkEnv := range linksPerStep {
			referenceLinkEnv = linkEnv
			referenceKeyID = keyID
			break
		}

		// Only one link, nothing to reduce, take the reference link
		if len(linksPerStep) == 1 {
			stepsMetadataReduced[step.Name] = referenceLinkEnv

			// Multiple links, reduce but first check
		} else {
			// Artifact maps must be equal for each type among all links
			// TODO: What should we do if there are more links, than the
			// threshold requires, but not all of them are equal? Right now we would
			// also error.
			for keyID, linkEnv := range linksPerStep {
				if !reflect.DeepEqual(linkEnv.GetPayload().(Link).Materials,
					referenceLinkEnv.GetPayload().(Link).Materials) ||
					!reflect.DeepEqual(linkEnv.GetPayload().(Link).Products,
						referenceLinkEnv.GetPayload().(Link).Products) {
					return nil, fmt.Errorf("link '%s' and '%s' have different"+
						" artifacts",
						fmt.Sprintf(LinkNameFormat, step.Name, referenceKeyID),
						fmt.Sprintf(LinkNameFormat, step.Name, keyID))
				}
			}
			// We haven't errored out, so we can reduce (i.e take the reference link)
			stepsMetadataReduced[step.Name] = referenceLinkEnv
		}
	}
	return stepsMetadataReduced, nil
}

/*
VerifyStepCommandAlignment (soft) verifies that for each step of the passed
layout the command executed, as per the passed link, matches the expected
command, as per the layout.  Soft verification means that, in case a command
does not align, a warning is issued.
*/
func VerifyStepCommandAlignment(layout Layout,
	stepsMetadata map[string]map[string]Metadata) {
	for _, step := range layout.Steps {
		linksPerStep, ok := stepsMetadata[step.Name]
		// We should never get here, layout verification must fail earlier
		if !ok || len(linksPerStep) < 1 {
			panic("Could not verify command alignment for step '" + step.Name +
				"', no link metadata found.")
		}

		for signerKeyID, linkEnv := range linksPerStep {
			expectedCommandS := strings.Join(step.ExpectedCommand, " ")
			executedCommandS := strings.Join(linkEnv.GetPayload().(Link).Command, " ")

			if expectedCommandS != executedCommandS {
				linkName := fmt.Sprintf(LinkNameFormat, step.Name, signerKeyID)
				fmt.Printf("WARNING: Expected command for step '%s' (%s) and command"+
					" reported by '%s' (%s) differ.\n",
					step.Name, expectedCommandS, linkName, executedCommandS)
			}
		}
	}
}

/*
LoadLayoutCertificates loads the root and intermediate CAs from the layout if in the layout.
This will be used to check signatures that were used to sign links but not configured
in the PubKeys section of the step.  No configured CAs means we don't want to allow this.
Returned CertPools will be empty in this case.
*/
func LoadLayoutCertificates(layout Layout, intermediatePems [][]byte) (*x509.CertPool, *x509.CertPool, error) {
	rootPool := x509.NewCertPool()
	for _, certPem := range layout.RootCas {
		ok := rootPool.AppendCertsFromPEM([]byte(certPem.KeyVal.Certificate))
		if !ok {
			return nil, nil, fmt.Errorf("failed to load root certificates for layout")
		}
	}

	intermediatePool := x509.NewCertPool()
	for _, intermediatePem := range layout.IntermediateCas {
		ok := intermediatePool.AppendCertsFromPEM([]byte(intermediatePem.KeyVal.Certificate))
		if !ok {
			return nil, nil, fmt.Errorf("failed to load intermediate certificates for layout")
		}
	}

	for _, intermediatePem := range intermediatePems {
		ok := intermediatePool.AppendCertsFromPEM(intermediatePem)
		if !ok {
			return nil, nil, fmt.Errorf("failed to load provided intermediate certificates")
		}
	}

	return rootPool, intermediatePool, nil
}

/*
VerifyLinkSignatureThesholds verifies that for each step of the passed layout,
there are at least Threshold links, validly signed by different authorized
functionaries.  The returned map of link metadata per steps contains only
links with valid signatures from distinct functionaries and has the format:

	{
		<step name> : {
		<key id>: Metablock,
		<key id>: Metablock,
		...
		},
		<step name> : {
		<key id>: Metablock,
		<key id>: Metablock,
		...
		}
		...
	}

If for any step of the layout there are not enough links available, the first
return value is an empty map of Metablock maps and the second return value is
the error.
*/
func VerifyLinkSignatureThesholds(layout Layout,
	stepsMetadata map[string]map[string]Metadata, rootCertPool, intermediateCertPool *x509.CertPool) (
	map[string]map[string]Metadata, error) {
	// This will stores links with valid signature from an authorized functionary
	// for all steps
	stepsMetadataVerified := make(map[string]map[string]Metadata)

	// Try to find enough (>= threshold) links each with a valid signature from
	// distinct authorized functionaries for each step
	for _, step := range layout.Steps {
		var stepErr error

		// This will store links with valid signature from an authorized
		// functionary for the given step
		linksPerStepVerified := make(map[string]Metadata)

		// Check if there are any links at all for the given step
		linksPerStep, ok := stepsMetadata[step.Name]
		if !ok || len(linksPerStep) < 1 {
			stepErr = fmt.Errorf("no links found")
		}

		// For each link corresponding to a step, check that the signer key was
		// authorized, the layout contains a verification key and the signature
		// verification passes.  Only good links are stored, to verify thresholds
		// below.
		isAuthorizedSignature := false
		for signerKeyID, linkEnv := range linksPerStep {
			for _, authorizedKeyID := range step.PubKeys {
				if signerKeyID == authorizedKeyID {
					if verifierKey, ok := layout.Keys[authorizedKeyID]; ok {
						if err := linkEnv.VerifySignature(verifierKey); err == nil {
							linksPerStepVerified[signerKeyID] = linkEnv
							isAuthorizedSignature = true
							break
						}
					}
				}
			}

			// If the signer's key wasn't in our step's pubkeys array, check the cert pool to
			// see if the key is known to us.
			if !isAuthorizedSignature {
				sig, err := linkEnv.GetSignatureForKeyID(signerKeyID)
				if err != nil {
					stepErr = err
					continue
				}

				cert, err := sig.GetCertificate()
				if err != nil {
					stepErr = err
					continue
				}

				// test certificate against the step's constraints to make sure it's a valid functionary
				err = step.CheckCertConstraints(cert, layout.RootCAIDs(), rootCertPool, intermediateCertPool)
				if err != nil {
					stepErr = err
					continue
				}

				err = linkEnv.VerifySignature(cert)
				if err != nil {
					stepErr = err
					continue
				}

				linksPerStepVerified[signerKeyID] = linkEnv
			}
		}

		// Store all good links for a step
		stepsMetadataVerified[step.Name] = linksPerStepVerified

		if len(linksPerStepVerified) < step.Threshold {
			linksPerStep := stepsMetadata[step.Name]
			return nil, fmt.Errorf("step '%s' requires '%d' link metadata file(s)."+
				" '%d' out of '%d' available link(s) have a valid signature from an"+
				" authorized signer: %v", step.Name, step.Threshold,
				len(linksPerStepVerified), len(linksPerStep), stepErr)
		}
	}
	return stepsMetadataVerified, nil
}

/*
LoadLinksForLayout loads for every Step of the passed Layout a Metablock
containing the corresponding Link.  A base path to a directory that contains
the links may be passed using linkDir.  Link file names are constructed,
using LinkNameFormat together with the corresponding step name and authorized
functionary key ids.  A map of link metadata is returned and has the following
format:

	{
		<step name> : {
			<key id>: Metablock,
			<key id>: Metablock,
			...
		},
		<step name> : {
		<key id>: Metablock,
		<key id>: Metablock,
		...
		}
		...
	}

If a link cannot be loaded at a constructed link name or is invalid, it is
ignored. Only a preliminary threshold check is performed, that is, if there
aren't at least Threshold links for any given step, the first return value
is an empty map of Metablock maps and the second return value is the error.
*/
func LoadLinksForLayout(layout Layout, linkDir string) (map[string]map[string]Metadata, error) {
	stepsMetadata := make(map[string]map[string]Metadata)

	for _, step := range layout.Steps {
		linksPerStep := make(map[string]Metadata)
		// Since we can verify against certificates belonging to a CA, we need to
		// load any possible links
		linkFiles, err := filepath.Glob(path.Join(linkDir, fmt.Sprintf(LinkGlobFormat, step.Name)))
		if err != nil {
			return nil, err
		}

		for _, linkPath := range linkFiles {
			linkEnv, err := LoadMetadata(linkPath)
			if err != nil {
				continue
			}

			// To get the full key from the metadata's signatures, we have to check
			// for one with the same short id...
			signerShortKeyID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(linkPath), step.Name+"."), ".link")
			for _, sig := range linkEnv.Sigs() {
				if strings.HasPrefix(sig.KeyID, signerShortKeyID) {
					linksPerStep[sig.KeyID] = linkEnv
					break
				}
			}
		}

		if len(linksPerStep) < step.Threshold {
			return nil, fmt.Errorf("step '%s' requires '%d' link metadata file(s),"+
				" found '%d'", step.Name, step.Threshold, len(linksPerStep))
		}

		stepsMetadata[step.Name] = linksPerStep
	}

	return stepsMetadata, nil
}

/*
VerifyLayoutExpiration verifies that the passed Layout has not expired.  It
returns an error if the (zulu) date in the Expires field is in the past.
*/
func VerifyLayoutExpiration(layout Layout) error {
	expires, err := time.Parse(ISO8601DateSchema, layout.Expires)
	if err != nil {
		return err
	}
	// Uses timezone of expires, i.e. UTC
	if time.Until(expires) < 0 {
		return fmt.Errorf("layout has expired on '%s'", expires)
	}
	return nil
}

/*
VerifyLayoutSignatures verifies for each key in the passed key map the
corresponding signature of the Layout in the passed Metablock's Signed field.
Signatures and keys are associated by key id.  If the key map is empty, or the
Metablock's Signature field does not have a signature for one or more of the
passed keys, or a matching signature is invalid, an error is returned.
*/
func VerifyLayoutSignatures(layoutEnv Metadata,
	layoutKeys map[string]Key) error {
	if len(layoutKeys) < 1 {
		return fmt.Errorf("layout verification requires at least one key")
	}

	for _, key := range layoutKeys {
		if err := layoutEnv.VerifySignature(key); err != nil {
			return err
		}
	}
	return nil
}

/*
GetSummaryLink merges the materials of the first step (as mentioned in the
layout) and the products of the last step and returns a new link. This link
reports the materials and products and summarizes the overall software supply
chain.
NOTE: The assumption is that the steps mentioned in the layout are to be
performed sequentially. So, the first step mentioned in the layout denotes what
comes into the supply chain and the last step denotes what goes out.
*/
func GetSummaryLink(layout Layout, stepsMetadataReduced map[string]Metadata,
	stepName string, useDSSE bool) (Metadata, error) {
	var summaryLink Link
	if len(layout.Steps) > 0 {
		firstStepLink := stepsMetadataReduced[layout.Steps[0].Name]
		lastStepLink := stepsMetadataReduced[layout.Steps[len(layout.Steps)-1].Name]

		summaryLink.Materials = firstStepLink.GetPayload().(Link).Materials
		summaryLink.Name = stepName
		summaryLink.Type = firstStepLink.GetPayload().(Link).Type

		summaryLink.Products = lastStepLink.GetPayload().(Link).Products
		summaryLink.ByProducts = lastStepLink.GetPayload().(Link).ByProducts
		// Using the last command of the sublayout as the command
		// of the summary link can be misleading. Is it necessary to
		// include all the commands executed as part of sublayout?
		summaryLink.Command = lastStepLink.GetPayload().(Link).Command
	}

	if useDSSE {
		env := &Envelope{}
		if err := env.SetPayload(summaryLink); err != nil {
			return nil, err
		}

		return env, nil
	}

	return &Metablock{Signed: summaryLink}, nil
}

/*
VerifySublayouts checks if any step in the supply chain is a sublayout, and if
so, recursively resolves it and replaces it with a summary link summarizing the
steps carried out in the sublayout.
*/
func VerifySublayouts(layout Layout,
	stepsMetadataVerified map[string]map[string]Metadata,
	superLayoutLinkPath string, intermediatePems [][]byte, lineNormalization bool) (map[string]map[string]Metadata, error) {
	for stepName, linkData := range stepsMetadataVerified {
		for keyID, metadata := range linkData {
			if _, ok := metadata.GetPayload().(Layout); ok {
				layoutKeys := make(map[string]Key)
				layoutKeys[keyID] = layout.Keys[keyID]

				sublayoutLinkDir := fmt.Sprintf(SublayoutLinkDirFormat,
					stepName, keyID)
				sublayoutLinkPath := filepath.Join(superLayoutLinkPath,
					sublayoutLinkDir)
				summaryLink, err := InTotoVerify(metadata, layoutKeys,
					sublayoutLinkPath, stepName, make(map[string]string), intermediatePems, lineNormalization)
				if err != nil {
					return nil, err
				}
				linkData[keyID] = summaryLink
			}

		}
	}
	return stepsMetadataVerified, nil
}

// TODO: find a better way than two helper functions for the replacer op

func substituteParamatersInSlice(replacer *strings.Replacer, slice []string) []string {
	newSlice := make([]string, 0)
	for _, item := range slice {
		newSlice = append(newSlice, replacer.Replace(item))
	}
	return newSlice
}

func substituteParametersInSliceOfSlices(replacer *strings.Replacer,
	slice [][]string) [][]string {
	newSlice := make([][]string, 0)
	for _, item := range slice {
		newSlice = append(newSlice, substituteParamatersInSlice(replacer,
			item))
	}
	return newSlice
}

/*
SubstituteParameters performs parameter substitution in steps and inspections
in the following fields:
- Expected Materials and Expected Products of both
- Run of inspections
- Expected Command of steps
The substitution marker is '{}' and the keyword within the braces is replaced
by a value found in the substitution map passed, parameterDictionary. The
layout with parameters substituted is returned to the calling function.
*/
func SubstituteParameters(layout Layout,
	parameterDictionary map[string]string) (Layout, error) {

	if len(parameterDictionary) == 0 {
		return layout, nil
	}

	parameters := make([]string, 0)

	re := regexp.MustCompile("^[a-zA-Z0-9_-]+$")

	for parameter, value := range parameterDictionary {
		parameterFormatCheck := re.MatchString(parameter)
		if !parameterFormatCheck {
			return layout, fmt.Errorf("invalid format for parameter")
		}

		parameters = append(parameters, "{"+parameter+"}")
		parameters = append(parameters, value)
	}

	replacer := strings.NewReplacer(parameters...)

	for i := range layout.Steps {
		layout.Steps[i].ExpectedMaterials = substituteParametersInSliceOfSlices(
			replacer, layout.Steps[i].ExpectedMaterials)
		layout.Steps[i].ExpectedProducts = substituteParametersInSliceOfSlices(
			replacer, layout.Steps[i].ExpectedProducts)
		layout.Steps[i].ExpectedCommand = substituteParamatersInSlice(replacer,
			layout.Steps[i].ExpectedCommand)
	}

	for i := range layout.Inspect {
		layout.Inspect[i].ExpectedMaterials =
			substituteParametersInSliceOfSlices(replacer,
				layout.Inspect[i].ExpectedMaterials)
		layout.Inspect[i].ExpectedProducts =
			substituteParametersInSliceOfSlices(replacer,
				layout.Inspect[i].ExpectedProducts)
		layout.Inspect[i].Run = substituteParamatersInSlice(replacer,
			layout.Inspect[i].Run)
	}

	return layout, nil
}

/*
InTotoVerify can be used to verify an entire software supply chain according to
the in-toto specification.  It requires the metadata of the root layout, a map
that contains public keys to verify the root layout signatures, a path to a
directory from where it can load link metadata files, which are treated as
signed evidence for the steps defined in the layout, a step name, and a
paramater dictionary used for parameter substitution. The step name only
matters for sublayouts, where it's important to associate the summary of that
step with a unique name. The verification routine is as follows:

1. Verify layout signature(s) using passed key(s)
2. Verify layout expiration date
3. Substitute parameters in layout
4. Load link metadata files for steps of layout
5. Verify signatures and signature thresholds for steps of layout
6. Verify sublayouts recursively
7. Verify command alignment for steps of layout (only warns)
8. Verify artifact rules for steps of layout
9. Execute inspection commands (generates link metadata for each inspection)
10. Verify artifact rules for inspections of layout

InTotoVerify returns a summary link wrapped in a Metablock object and an error
value. If any of the verification routines fail, verification is aborted and
error is returned. In such an instance, the first value remains an empty
Metablock object.

NOTE: Artifact rules of type "create", "modify"
and "delete" are currently not supported.
*/
func InTotoVerify(layoutEnv Metadata, layoutKeys map[string]Key,
	linkDir string, stepName string, parameterDictionary map[string]string, intermediatePems [][]byte, lineNormalization bool) (
	Metadata, error) {

	// Verify root signatures
	if err := VerifyLayoutSignatures(layoutEnv, layoutKeys); err != nil {
		return nil, err
	}

	useDSSE := false
	if _, ok := layoutEnv.(*Envelope); ok {
		useDSSE = true
	}

	// Extract the layout from its Metadata container (for further processing)
	layout, ok := layoutEnv.GetPayload().(Layout)
	if !ok {
		return nil, ErrNotLayout
	}

	// Verify layout expiration
	if err := VerifyLayoutExpiration(layout); err != nil {
		return nil, err
	}

	// Substitute parameters in layout
	layout, err := SubstituteParameters(layout, parameterDictionary)
	if err != nil {
		return nil, err
	}

	rootCertPool, intermediateCertPool, err := LoadLayoutCertificates(layout, intermediatePems)
	if err != nil {
		return nil, err
	}

	// Load links for layout
	stepsMetadata, err := LoadLinksForLayout(layout, linkDir)
	if err != nil {
		return nil, err
	}

	// Verify link signatures
	stepsMetadataVerified, err := VerifyLinkSignatureThesholds(layout,
		stepsMetadata, rootCertPool, intermediateCertPool)
	if err != nil {
		return nil, err
	}

	// Verify and resolve sublayouts
	stepsSublayoutVerified, err := VerifySublayouts(layout,
		stepsMetadataVerified, linkDir, intermediatePems, lineNormalization)
	if err != nil {
		return nil, err
	}

	// Verify command alignment (WARNING only)
	VerifyStepCommandAlignment(layout, stepsSublayoutVerified)

	// Given that signature thresholds have been checked above and the rest of
	// the relevant link properties, i.e. materials and products, have to be
	// exactly equal, we can reduce the map of steps metadata. However, we error
	// if the relevant properties are not equal among links of a step.
	stepsMetadataReduced, err := ReduceStepsMetadata(layout,
		stepsSublayoutVerified)
	if err != nil {
		return nil, err
	}

	// Verify artifact rules
	if err = VerifyArtifacts(layout.stepsAsInterfaceSlice(),
		stepsMetadataReduced); err != nil {
		return nil, err
	}

	inspectionMetadata, err := RunInspections(layout, "", lineNormalization, useDSSE)
	if err != nil {
		return nil, err
	}

	// Add steps metadata to inspection metadata, because inspection artifact
	// rules may also refer to artifacts reported by step links
	for k, v := range stepsMetadataReduced {
		inspectionMetadata[k] = v
	}

	if err = VerifyArtifacts(layout.inspectAsInterfaceSlice(),
		inspectionMetadata); err != nil {
		return nil, err
	}

	summaryLink, err := GetSummaryLink(layout, stepsMetadataReduced, stepName, useDSSE)
	if err != nil {
		return nil, err
	}

	return summaryLink, nil
}

/*
InTotoVerifyWithDirectory provides the same functionality as InTotoVerify, but
adds the possibility to select a local directory from where the inspections are run.
*/
func InTotoVerifyWithDirectory(layoutEnv Metadata, layoutKeys map[string]Key,
	linkDir string, runDir string, stepName string, parameterDictionary map[string]string, intermediatePems [][]byte, lineNormalization bool) (
	Metadata, error) {

	// runDir sanity checks
	// check if path exists
	info, err := os.Stat(runDir)
	if err != nil {
		return nil, err
	}

	// check if runDir is a symlink
	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		return nil, ErrInspectionRunDirIsSymlink
	}

	// check if runDir is writable and a directory
	err = isWritable(runDir)
	if err != nil {
		return nil, err
	}

	// check if runDir is empty (we do not want to overwrite files)
	// We abuse File.Readdirnames for this action.
	f, err := os.Open(runDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// We use Readdirnames(1) for performance reasons, one child node
	// is enough to proof that the directory is not empty
	_, err = f.Readdirnames(1)
	// if io.EOF gets returned as error the directory is empty
	if err == io.EOF {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	// Verify root signatures
	if err := VerifyLayoutSignatures(layoutEnv, layoutKeys); err != nil {
		return nil, err
	}

	useDSSE := false
	if _, ok := layoutEnv.(*Envelope); ok {
		useDSSE = true
	}

	// Extract the layout from its Metadata container (for further processing)
	layout, ok := layoutEnv.GetPayload().(Layout)
	if !ok {
		return nil, ErrNotLayout
	}

	// Verify layout expiration
	if err := VerifyLayoutExpiration(layout); err != nil {
		return nil, err
	}

	// Substitute parameters in layout
	layout, err = SubstituteParameters(layout, parameterDictionary)
	if err != nil {
		return nil, err
	}

	rootCertPool, intermediateCertPool, err := LoadLayoutCertificates(layout, intermediatePems)
	if err != nil {
		return nil, err
	}

	// Load links for layout
	stepsMetadata, err := LoadLinksForLayout(layout, linkDir)
	if err != nil {
		return nil, err
	}

	// Verify link signatures
	stepsMetadataVerified, err := VerifyLinkSignatureThesholds(layout,
		stepsMetadata, rootCertPool, intermediateCertPool)
	if err != nil {
		return nil, err
	}

	// Verify and resolve sublayouts
	stepsSublayoutVerified, err := VerifySublayouts(layout,
		stepsMetadataVerified, linkDir, intermediatePems, lineNormalization)
	if err != nil {
		return nil, err
	}

	// Verify command alignment (WARNING only)
	VerifyStepCommandAlignment(layout, stepsSublayoutVerified)

	// Given that signature thresholds have been checked above and the rest of
	// the relevant link properties, i.e. materials and products, have to be
	// exactly equal, we can reduce the map of steps metadata. However, we error
	// if the relevant properties are not equal among links of a step.
	stepsMetadataReduced, err := ReduceStepsMetadata(layout,
		stepsSublayoutVerified)
	if err != nil {
		return nil, err
	}

	// Verify artifact rules
	if err = VerifyArtifacts(layout.stepsAsInterfaceSlice(),
		stepsMetadataReduced); err != nil {
		return nil, err
	}

	inspectionMetadata, err := RunInspections(layout, runDir, lineNormalization, useDSSE)
	if err != nil {
		return nil, err
	}

	// Add steps metadata to inspection metadata, because inspection artifact
	// rules may also refer to artifacts reported by step links
	for k, v := range stepsMetadataReduced {
		inspectionMetadata[k] = v
	}

	if err = VerifyArtifacts(layout.inspectAsInterfaceSlice(),
		inspectionMetadata); err != nil {
		return nil, err
	}

	summaryLink, err := GetSummaryLink(layout, stepsMetadataReduced, stepName, useDSSE)
	if err != nil {
		return nil, err
	}

	return summaryLink, nil
}
