* Why is this change needed or what are the use cases?

   Different registries may want to authenticate their users in ways that aren't currently (well) supported. This may include, for example, 2-factor authentication, or auth flows which differ depending on the context.

   For example, GCR currently requires for the Docker commands to be wrapped in 'gcloud docker' in order for requests to be elegantly authenticated. Allowing for a registry-specific credential helper would allow for us to deprecate this scheme.

* What are the requirements this change should meet?

   Registries should be able to provide their users with an authentication plugin which can provide the Docker client with that user's registry-specific credentials. This plugin framework should work well with the existing credential stores, and several should be configurable at a given time so that the user can work with multiple respoitories from the same client without having to modify their docker config every time a context switch happens.

* What are some ways to design/implement this feature?

   Allowing the user to specify a binary in a similar way to the existing credential store scheme would be the path of least resistance. This field could be added into types.AuthConfig, or into an entirely new field in top level of the docker config. Modifying "credsStore" to be a map would be unnecessary and introduce too many deserialization headaches.

* Which design/implementation do you think is best and why?

   Adding a new field to the top level of the docker config would most likely be the path of least resistance; preserving the credsStore to act as the default credential store as well as being overall less complex to implement compared to modifying types.AuthConfig.

   e.g. config.json:

   ```{
	...
	
	"credentialHelpers": {
		"gcr.io": "gcr"
	}

	...
   }```


* What are the risks or limitations of your proposal?

   Confusion over the order of precedence for credential helper vs. "credsStore" vs. "auths" might be a bit of a headache, but it would not be difficult for credential helper implementers to create a script to inject their specific configuration into the user's docker config, bypassing the human element entirely.
