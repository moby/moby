# Changes

## [1.8.1](https://github.com/googleapis/google-cloud-go/compare/logging/v1.8.0...logging/v1.8.1) (2023-08-14)


### Bug Fixes

* **logging:** Init default retryer ([#8415](https://github.com/googleapis/google-cloud-go/issues/8415)) ([c980708](https://github.com/googleapis/google-cloud-go/commit/c980708c5f69f69c21632250a96f4f2c2e87f697))

## [1.8.0](https://github.com/googleapis/google-cloud-go/compare/logging/v1.7.0...logging/v1.8.0) (2023-08-09)


### Features

* **logging:** Log Analytics features of the Cloud Logging API feat: Add ConfigServiceV2.CreateBucketAsync method for creating Log Buckets asynchronously feat: Add ConfigServiceV2.UpdateBucketAsync method for creating Log Buckets asynchronously feat: Add ConfigServiceV2.CreateLink method for creating linked datasets for Log Analytics Buckets feat: Add ConfigServiceV2.DeleteLink method for deleting linked datasets feat: Add ConfigServiceV2.ListLinks method for listing linked datasets feat: Add ConfigServiceV2.GetLink methods for describing linked datasets feat: Add LogBucket.analytics_enabled field that specifies whether Log Bucket's Analytics features are enabled feat: Add LogBucket.index_configs field that contains a list of Log Bucket's indexed fields and related configuration data docs: Documentation for the Log Analytics features of the Cloud Logging API ([31c3766](https://github.com/googleapis/google-cloud-go/commit/31c3766c9c4cab411669c14fc1a30bd6d2e3f2dd))
* **logging:** Update all direct dependencies ([b340d03](https://github.com/googleapis/google-cloud-go/commit/b340d030f2b52a4ce48846ce63984b28583abde6))


### Bug Fixes

* **logging/logadmin:** Fix paging example filter ([#8224](https://github.com/googleapis/google-cloud-go/issues/8224)) ([710c627](https://github.com/googleapis/google-cloud-go/commit/710c627b2cf46b8b2e83ff02e020700b3281e498))
* **logging:** REST query UpdateMask bug ([df52820](https://github.com/googleapis/google-cloud-go/commit/df52820b0e7721954809a8aa8700b93c5662dc9b))
* **logging:** Update grpc to v1.55.0 ([1147ce0](https://github.com/googleapis/google-cloud-go/commit/1147ce02a990276ca4f8ab7a1ab65c14da4450ef))
* **logging:** Use fieldmask directly instead of field_mask genproto alias ([#8031](https://github.com/googleapis/google-cloud-go/issues/8031)) ([13d9483](https://github.com/googleapis/google-cloud-go/commit/13d9483ddcfef20ea6dcdb3db5f4560c11c15c09))

## [1.7.0](https://github.com/googleapis/google-cloud-go/compare/logging/v1.6.1...logging/v1.7.0) (2023-02-27)


### Features

* **logging:** Add (*Logger). StandardLoggerFromTemplate() method. ([#7261](https://github.com/googleapis/google-cloud-go/issues/7261)) ([533ecbb](https://github.com/googleapis/google-cloud-go/commit/533ecbb19a2833e667ad139a6604fd40dfb43cdc))
* **logging:** Add REST client ([06a54a1](https://github.com/googleapis/google-cloud-go/commit/06a54a16a5866cce966547c51e203b9e09a25bc0))
* **logging:** Rewrite signatures and type in terms of new location ([620e6d8](https://github.com/googleapis/google-cloud-go/commit/620e6d828ad8641663ae351bfccfe46281e817ad))


### Bug Fixes

* **logging:** Correctly populate SourceLocation when logging via (*Logger).StandardLogger ([#7320](https://github.com/googleapis/google-cloud-go/issues/7320)) ([1a0bd13](https://github.com/googleapis/google-cloud-go/commit/1a0bd13b88569826f4ee6528e9cdb59fd26914fa))
* **logging:** Fix typo in README.md ([#7297](https://github.com/googleapis/google-cloud-go/issues/7297)) ([82aa2ee](https://github.com/googleapis/google-cloud-go/commit/82aa2ee9381f793bd731f1b6789fc18e4b671bd7))

## [1.6.1](https://github.com/googleapis/google-cloud-go/compare/logging/v1.6.0...logging/v1.6.1) (2022-12-02)


### Bug Fixes

* **logging:** downgrade some dependencies ([7540152](https://github.com/googleapis/google-cloud-go/commit/754015236d5af7c82a75da218b71a87b9ead6eb5))

## [1.6.0](https://github.com/googleapis/google-cloud-go/compare/logging/v1.5.0...logging/v1.6.0) (2022-11-29)


### Features

* **logging:** start generating proto stubs ([0eb700d](https://github.com/googleapis/google-cloud-go/commit/0eb700d17c4cac56f59038f0f3ae5a65257a3d38))


### Bug Fixes

* **logging:** Fix stdout log http request format ([#7083](https://github.com/googleapis/google-cloud-go/issues/7083)) ([2894e66](https://github.com/googleapis/google-cloud-go/commit/2894e66be7ff7536f725ede453d1834586a361bd))

## [1.5.0](https://github.com/googleapis/google-cloud-go/compare/logging/v1.4.2...logging/v1.5.0) (2022-06-25)


### Features

* **logging:** add better version metadata to calls ([d1ad921](https://github.com/googleapis/google-cloud-go/commit/d1ad921d0322e7ce728ca9d255a3cf0437d26add))
* **logging:** set versionClient to module version ([55f0d92](https://github.com/googleapis/google-cloud-go/commit/55f0d92bf112f14b024b4ab0076c9875a17423c9))
* **logging:** support structured logging functionality ([#6029](https://github.com/googleapis/google-cloud-go/issues/6029)) ([56f4cdd](https://github.com/googleapis/google-cloud-go/commit/56f4cdd066cc9eaeece2c6fb466d58c3e7c41563))
* **logging:** Update Logging API with latest changes ([5af548b](https://github.com/googleapis/google-cloud-go/commit/5af548bee4ffde279727b2e1ad9b072925106a74))


### Bug Fixes

* **logging:** remove instance_name resource label ([#5461](https://github.com/googleapis/google-cloud-go/issues/5461)) ([115385f](https://github.com/googleapis/google-cloud-go/commit/115385f066ee54cf35a093749bc2673a17b3fa08))

### [1.4.2](https://www.github.com/googleapis/google-cloud-go/compare/logging/v1.4.1...logging/v1.4.2) (2021-05-20)


### Bug Fixes

* **logging:** correctly detect GKE resource ([#4092](https://www.github.com/googleapis/google-cloud-go/issues/4092)) ([a2538e1](https://www.github.com/googleapis/google-cloud-go/commit/a2538e16123c21da62036b56df8c104360f1c2d6))

### [1.4.1](https://www.github.com/googleapis/google-cloud-go/compare/logging/v1.4.0...logging/v1.4.1) (2021-05-03)


### Bug Fixes

* **logging:** allow nil or custom zones in resource detection ([#3997](https://www.github.com/googleapis/google-cloud-go/issues/3997)) ([aded90b](https://www.github.com/googleapis/google-cloud-go/commit/aded90b92de3fa3bed079af1aa4879d00572e8ae))
* **logging:** appengine zone label ([#3998](https://www.github.com/googleapis/google-cloud-go/issues/3998)) ([394a586](https://www.github.com/googleapis/google-cloud-go/commit/394a586bac04953e92a6496a7ca3b61bd64155ab))

## [1.4.0](https://www.github.com/googleapis/google-cloud-go/compare/logging/v1.2.0...logging/v1.4.0) (2021-04-15)


### Features

* **logging:** cloud run and functions resource autodetection ([#3909](https://www.github.com/googleapis/google-cloud-go/issues/3909)) ([1204de8](https://www.github.com/googleapis/google-cloud-go/commit/1204de85e58334bf93fecdcb0ab8b581449c2745))
* **logging:** make toLogEntry function public ([#3863](https://www.github.com/googleapis/google-cloud-go/issues/3863)) ([71828c2](https://www.github.com/googleapis/google-cloud-go/commit/71828c28d424c34da6d0392651739a364cd57e79))


### Bug Fixes

* **logging:** Entries has a 24H default filter ([#3120](https://www.github.com/googleapis/google-cloud-go/issues/3120)) ([b32eb82](https://www.github.com/googleapis/google-cloud-go/commit/b32eb822d17838bde91c610a5a9d392d325a592d))

## v1.3.0

- Updates to various dependencies.

## [1.2.0](https://www.github.com/googleapis/google-cloud-go/compare/logging/v1.1.2...v1.2.0) (2021-01-25)


### Features

* **logging:** add localIP and Cache fields to HTTPRequest conversion from proto ([#3600](https://www.github.com/googleapis/google-cloud-go/issues/3600)) ([f93027b](https://www.github.com/googleapis/google-cloud-go/commit/f93027b47735e7c181989666e0826bea57ec51e1))

### [1.1.2](https://www.github.com/googleapis/google-cloud-go/compare/logging/v1.1.1...v1.1.2) (2020-11-09)


### Bug Fixes

* **logging:** allow X-Cloud-Trace-Context fields to be optional ([#3062](https://www.github.com/googleapis/google-cloud-go/issues/3062)) ([7ff03cf](https://www.github.com/googleapis/google-cloud-go/commit/7ff03cf9a544e753de5b034e18339ecf517d2193))
* **logging:** do not panic in library code ([#3076](https://www.github.com/googleapis/google-cloud-go/issues/3076)) ([529be97](https://www.github.com/googleapis/google-cloud-go/commit/529be977f766443f49cb8914e17ba07c93841e84)), closes [#1862](https://www.github.com/googleapis/google-cloud-go/issues/1862)

## v1.1.1

- Rebrand "Stackdriver Logging" to "Cloud Logging".

## v1.1.0

- Support unmarshalling stringified Severity.
- Add exported SetGoogleClientInfo wrappers to manual file.
- Support no payload.
- Update "Grouping Logs by Request" docs.
- Add auto-detection of monitored resources on GAE Standard.

## v1.0.0

This is the first tag to carve out logging as its own module. See:
https://github.com/golang/go/wiki/Modules#is-it-possible-to-add-a-module-to-a-multi-module-repository.
