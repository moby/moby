**Why is this change needed or what are the use cases?**
There are currently user-configurable properties which require manual manipulation of the docker config in order to set. This is clunky and error prone; ideally the docker config would be 100% managed by the Docker client itself.

**What are the requirements this change should meet?**
All properties which are user-configurable should have a CLI mechanism to properly validate and persist in the docker config.
These settings currently include (at least) the following:
* HttpHeaders
* psFormat
* imagesFormat
* detachKeys
* credsStore

**What are some ways to design/implement this feature?**
* Emulate existing command-line flags, but prefix the command with 'docker config'
  * e.g. `docker config ps --format ...`
* Design a [surface similar to `git config`](https://git-scm.com/docs/git-config), i.e. reference properties via JSON-like path, use flags or sub-commands to specify operations other than set/get.
  * `docker config psFormat "table {{.ID}}\\t{{.Image}}\\t{{.Command}}\\t{{.Labels}}"`
  * `docker config psFormat` (returns the set value)
  * `docker config --unset psFormat`

**Which design/implementation do you think is best and why?**
I like the git-config approach because it's likely both familiar to users and provides easy extensibility for more complex operations (such as appending a new HTTP header to an existing list). The downside is that there isn't a 1:1 mapping between flags and properties in the config (e.g. `docker ps --format`, `docker images --format`), so there might be a slight learning curve. The upside is that we'd have the ability to validate and fail-fast any invalid properties/values.
