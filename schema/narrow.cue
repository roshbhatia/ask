package provider

#Manifest: {
	actions: "provider.validate"!: _
	if actions["inference.generate"] == _|_ {
		actions: "inference.evaluate"!: _
	}
}
