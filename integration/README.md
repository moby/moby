## Legacy integration tests

`./integration` contains Docker's legacy integration tests.
It is DEPRECATED and will eventually be removed.

### If you are a *CONTRIBUTOR* and want to add a test:

* Consider mocking out side effects and contributing a *unit test* in the subsystem
you're modifying. For example, the remote API has unit tests in `./api/server/server_unit_tests.go`.
The events subsystem has unit tests in `./events/events_test.go`. And so on.

* For end-to-end integration tests, please contribute to `./integration-cli`.


### If you are a *MAINTAINER*

Please don't allow patches adding new tests to `./integration`.

### If you are *LOOKING FOR A WAY TO HELP*

Please consider porting tests away from `./integration` and into either unit tests or CLI tests.

Any help will be greatly appreciated!
