// Package provider narrows the canonical provider/v1 contract to Ask.
//
// The contract itself is provider.cue in roshbhatia/provider-spec, pinned as
// the provider-spec flake input. This file adds only Ask's rule: every
// provider answers inference.generate and provider.validate. Vet a manifest
// with both files:
//
//	cue vet -d '#Manifest' "$PROVIDER_SPEC/provider.cue" schema/narrow.cue extras/claude/provider.yaml
package provider

#Manifest: actions: {
	"inference.generate"!: _
	"provider.validate"!:  _
}
