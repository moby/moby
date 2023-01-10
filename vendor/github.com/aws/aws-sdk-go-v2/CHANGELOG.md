# Release (2022-08-31)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.20.2](service/cloudfront/CHANGELOG.md#v1202-2022-08-31)
  * **Documentation**: Update API documentation for CloudFront origin access control (OAC)
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.15.0](service/identitystore/CHANGELOG.md#v1150-2022-08-31)
  * **Feature**: Expand IdentityStore API to support Create, Read, Update, Delete and Get operations for User, Group and GroupMembership resources.
* `github.com/aws/aws-sdk-go-v2/service/iotthingsgraph`: [v1.13.0](service/iotthingsgraph/CHANGELOG.md#v1130-2022-08-31)
  * **Feature**: This release deprecates all APIs of the ThingsGraph service
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.18.0](service/ivs/CHANGELOG.md#v1180-2022-08-31)
  * **Feature**: IVS Merge Fragmented Streams. This release adds support for recordingReconnectWindow field in IVS recordingConfigurations. For more information see https://docs.aws.amazon.com/ivs/latest/APIReference/Welcome.html
* `github.com/aws/aws-sdk-go-v2/service/rdsdata`: [v1.12.12](service/rdsdata/CHANGELOG.md#v11212-2022-08-31)
  * **Documentation**: Documentation updates for RDS Data API
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.40.0](service/sagemaker/CHANGELOG.md#v1400-2022-08-31)
  * **Feature**: SageMaker Inference Recommender now accepts Inference Recommender fields: Domain, Task, Framework, SamplePayloadUrl, SupportedContentTypes, SupportedInstanceTypes, directly in our CreateInferenceRecommendationsJob API through ContainerConfig

# Release (2022-08-30)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.17.0](service/greengrassv2/CHANGELOG.md#v1170-2022-08-30)
  * **Feature**: Adds topologyFilter to ListInstalledComponentsRequest which allows filtration of components by ROOT or ALL (including root and dependency components). Adds lastStatusChangeTimestamp to ListInstalledComponents response to show the last time a component changed state on a device.
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.14.15](service/identitystore/CHANGELOG.md#v11415-2022-08-30)
  * **Documentation**: Documentation updates for the Identity Store CLI Reference.
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.15.0](service/lookoutequipment/CHANGELOG.md#v1150-2022-08-30)
  * **Feature**: This release adds new apis for providing labels.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.23.0](service/macie2/CHANGELOG.md#v1230-2022-08-30)
  * **Feature**: This release of the Amazon Macie API adds support for using allow lists to define specific text and text patterns to ignore when inspecting data sources for sensitive data.
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.11.19](service/sso/CHANGELOG.md#v11119-2022-08-30)
  * **Documentation**: Documentation updates for the AWS IAM Identity Center Portal CLI Reference.
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.15.7](service/ssoadmin/CHANGELOG.md#v1157-2022-08-30)
  * **Documentation**: Documentation updates for the AWS IAM Identity Center CLI Reference.

# Release (2022-08-29)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.24.9](service/fsx/CHANGELOG.md#v1249-2022-08-29)
  * **Documentation**: Documentation updates for Amazon FSx for NetApp ONTAP.
* `github.com/aws/aws-sdk-go-v2/service/voiceid`: [v1.11.0](service/voiceid/CHANGELOG.md#v1110-2022-08-29)
  * **Feature**: Amazon Connect Voice ID now detects voice spoofing.  When a prospective fraudster tries to spoof caller audio using audio playback or synthesized speech, Voice ID will return a risk score and outcome to indicate the how likely it is that the voice is spoofed.

# Release (2022-08-26)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.18.0](service/mediapackage/CHANGELOG.md#v1180-2022-08-26)
  * **Feature**: This release adds Ads AdTriggers and AdsOnDeliveryRestrictions to describe calls for CMAF endpoints on MediaPackage.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.25.1](service/rds/CHANGELOG.md#v1251-2022-08-26)
  * **Documentation**: Removes support for RDS Custom from DBInstanceClass in ModifyDBInstance

# Release (2022-08-25)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.18.13](service/elasticloadbalancingv2/CHANGELOG.md#v11813-2022-08-25)
  * **Documentation**: Documentation updates for ELBv2.  Gateway Load Balancer now supports Configurable Flow Stickiness, enabling you to configure the hashing used to maintain stickiness of flows to a specific target appliance.
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.15.0](service/gamelift/CHANGELOG.md#v1150-2022-08-25)
  * **Feature**: This release adds support for eight EC2 local zones as fleet locations; Atlanta, Chicago, Dallas, Denver, Houston, Kansas City (us-east-1-mci-1a), Los Angeles, and Phoenix. It also adds support for C5d, C6a, C6i, and R5d EC2 instance families.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.22.0](service/iotwireless/CHANGELOG.md#v1220-2022-08-25)
  * **Feature**: This release includes a new feature for the customers to enable the LoRa gateways to send out beacons for Class B devices and an option to select one or more gateways for Class C devices when sending the LoRaWAN downlink messages.
* `github.com/aws/aws-sdk-go-v2/service/ivschat`: [v1.0.13](service/ivschat/CHANGELOG.md#v1013-2022-08-25)
  * **Documentation**: Documentation change for IVS Chat API Reference. Doc-only update to add a paragraph on ARNs to the Welcome section.
* `github.com/aws/aws-sdk-go-v2/service/panorama`: [v1.8.0](service/panorama/CHANGELOG.md#v180-2022-08-25)
  * **Feature**: Support sorting and filtering in ListDevices API, and add more fields to device listings and single device detail
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.13.0](service/ssooidc/CHANGELOG.md#v1130-2022-08-25)
  * **Feature**: Updated required request parameters on IAM Identity Center's OIDC CreateToken action.

# Release (2022-08-24)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.20.0](service/cloudfront/CHANGELOG.md#v1200-2022-08-24)
  * **Feature**: Adds support for CloudFront origin access control (OAC), making it possible to restrict public access to S3 bucket origins in all AWS Regions, those with SSE-KMS, and more.
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.25.0](service/configservice/CHANGELOG.md#v1250-2022-08-24)
  * **Feature**: AWS Config now supports ConformancePackTemplate documents in SSM Docs for the deployment and update of conformance packs.
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.18.14](service/iam/CHANGELOG.md#v11814-2022-08-24)
  * **Documentation**: Documentation updates for AWS Identity and Access Management (IAM).
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.17.1](service/ivs/CHANGELOG.md#v1171-2022-08-24)
  * **Documentation**: Documentation Change for IVS API Reference - Doc-only update to type field description for CreateChannel and UpdateChannel actions and for Channel data type. Also added Amazon Resource Names (ARNs) paragraph to Welcome section.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.24.0](service/quicksight/CHANGELOG.md#v1240-2022-08-24)
  * **Feature**: Added a new optional property DashboardVisual under ExperienceConfiguration parameter of GenerateEmbedUrlForAnonymousUser and GenerateEmbedUrlForRegisteredUser API operations. This supports embedding of specific visuals in QuickSight dashboards.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.21.5](service/transfer/CHANGELOG.md#v1215-2022-08-24)
  * **Documentation**: Documentation updates for AWS Transfer Family

# Release (2022-08-23)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.25.0](service/rds/CHANGELOG.md#v1250-2022-08-23)
  * **Feature**: RDS for Oracle supports Oracle Data Guard switchover and read replica backups.
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.15.5](service/ssoadmin/CHANGELOG.md#v1155-2022-08-23)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)

# Release (2022-08-22)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.19.5](service/docdb/CHANGELOG.md#v1195-2022-08-22)
  * **Documentation**: Update document for volume clone
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.54.0](service/ec2/CHANGELOG.md#v1540-2022-08-22)
  * **Feature**: R6a instances are powered by 3rd generation AMD EPYC (Milan) processors delivering all-core turbo frequency of 3.6 GHz. C6id, M6id, and R6id instances are powered by 3rd generation Intel Xeon Scalable processor (Ice Lake) delivering all-core turbo frequency of 3.5 GHz.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.23.0](service/forecast/CHANGELOG.md#v1230-2022-08-22)
  * **Feature**: releasing What-If Analysis APIs and update ARN regex pattern to be more strict in accordance with security recommendation
* `github.com/aws/aws-sdk-go-v2/service/forecastquery`: [v1.12.0](service/forecastquery/CHANGELOG.md#v1120-2022-08-22)
  * **Feature**: releasing What-If Analysis APIs
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.24.0](service/iotsitewise/CHANGELOG.md#v1240-2022-08-22)
  * **Feature**: Enable non-unique asset names under different hierarchies
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.23.0](service/lexmodelsv2/CHANGELOG.md#v1230-2022-08-22)
  * **Feature**: This release introduces a new feature to stop a running BotRecommendation Job for Automated Chatbot Designer.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.23.0](service/securityhub/CHANGELOG.md#v1230-2022-08-22)
  * **Feature**: Added new resource details objects to ASFF, including resources for AwsBackupBackupVault, AwsBackupBackupPlan and AwsBackupRecoveryPoint. Added FixAvailable, FixedInVersion and Remediation  to Vulnerability.
* `github.com/aws/aws-sdk-go-v2/service/supportapp`: [v1.0.0](service/supportapp/CHANGELOG.md#v100-2022-08-22)
  * **Release**: New AWS service client module
  * **Feature**: This is the initial SDK release for the AWS Support App in Slack.

# Release (2022-08-19)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.28.0](service/connect/CHANGELOG.md#v1280-2022-08-19)
  * **Feature**: This release adds SearchSecurityProfiles API which can be used to search for Security Profile resources within a Connect Instance.
* `github.com/aws/aws-sdk-go-v2/service/ivschat`: [v1.0.12](service/ivschat/CHANGELOG.md#v1012-2022-08-19)
  * **Documentation**: Documentation Change for IVS Chat API Reference - Doc-only update to change text/description for tags field.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.33.0](service/kendra/CHANGELOG.md#v1330-2022-08-19)
  * **Feature**: This release adds support for a new authentication type - Personal Access Token (PAT) for confluence server.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.17.0](service/lookoutmetrics/CHANGELOG.md#v1170-2022-08-19)
  * **Feature**: This release is to make GetDataQualityMetrics API publicly available.

# Release (2022-08-18)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmediapipelines`: [v1.1.0](service/chimesdkmediapipelines/CHANGELOG.md#v110-2022-08-18)
  * **Feature**: The Amazon Chime SDK now supports live streaming of real-time video from the Amazon Chime SDK sessions to streaming platforms such as Amazon IVS and Amazon Elemental MediaLive. We have also added support for concatenation to create a single media capture file.
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.21.0](service/cloudwatch/CHANGELOG.md#v1210-2022-08-18)
  * **Feature**: Add support for managed Contributor Insights Rules
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.18.4](service/cognitoidentityprovider/CHANGELOG.md#v1184-2022-08-18)
  * **Documentation**: This change is being made simply to fix the public documentation based on the models. We have included the PasswordChange and ResendCode events, along with the Pass, Fail and InProgress status. We have removed the Success and Failure status which are never returned by our APIs.
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.16.0](service/dynamodb/CHANGELOG.md#v1160-2022-08-18)
  * **Feature**: This release adds support for importing data from S3 into a new DynamoDB table
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.53.0](service/ec2/CHANGELOG.md#v1530-2022-08-18)
  * **Feature**: This release adds support for VPN log options , a new feature allowing S2S VPN connections to send IKE activity logs to CloudWatch Logs
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.15.0](service/networkmanager/CHANGELOG.md#v1150-2022-08-18)
  * **Feature**: Add TransitGatewayPeeringAttachmentId property to TransitGatewayPeering Model

# Release (2022-08-17)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.15.0](service/appmesh/CHANGELOG.md#v1150-2022-08-17)
  * **Feature**: AWS App Mesh release to support Multiple Listener and Access Log Format feature
* `github.com/aws/aws-sdk-go-v2/service/connectcampaigns`: [v1.1.0](service/connectcampaigns/CHANGELOG.md#v110-2022-08-17)
  * **Feature**: Updated exceptions for Amazon Connect Outbound Campaign api's.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.32.0](service/kendra/CHANGELOG.md#v1320-2022-08-17)
  * **Feature**: This release adds Zendesk connector (which allows you to specify Zendesk SAAS platform as data source), Proxy Support for Sharepoint and Confluence Server (which allows you to specify the proxy configuration if proxy is required to connect to your Sharepoint/Confluence Server as data source).
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.17.0](service/lakeformation/CHANGELOG.md#v1170-2022-08-17)
  * **Feature**: This release adds a new API support "AssumeDecoratedRoleWithSAML" and also release updates the corresponding documentation.
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.24.0](service/lambda/CHANGELOG.md#v1240-2022-08-17)
  * **Feature**: Added support for customization of Consumer Group ID for MSK and Kafka Event Source Mappings.
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.22.0](service/lexmodelsv2/CHANGELOG.md#v1220-2022-08-17)
  * **Feature**: This release introduces support for enhanced conversation design with the ability to define custom conversation flows with conditional branching and new bot responses.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.24.0](service/rds/CHANGELOG.md#v1240-2022-08-17)
  * **Feature**: Adds support for Internet Protocol Version 6 (IPv6) for RDS Aurora database clusters.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.18](service/secretsmanager/CHANGELOG.md#v11518-2022-08-17)
  * **Documentation**: Documentation updates for Secrets Manager.

# Release (2022-08-16)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.20.0](service/rekognition/CHANGELOG.md#v1200-2022-08-16)
  * **Feature**: This release adds APIs which support copying an Amazon Rekognition Custom Labels model and managing project policies across AWS account.
* `github.com/aws/aws-sdk-go-v2/service/servicecatalog`: [v1.14.12](service/servicecatalog/CHANGELOG.md#v11412-2022-08-16)
  * **Documentation**: Documentation updates for Service Catalog

# Release (2022-08-15)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.19.0](service/cloudfront/CHANGELOG.md#v1190-2022-08-15)
  * **Feature**: Adds Http 3 support to distributions
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.14.13](service/identitystore/CHANGELOG.md#v11413-2022-08-15)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.11.17](service/sso/CHANGELOG.md#v11117-2022-08-15)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.9.0](service/wisdom/CHANGELOG.md#v190-2022-08-15)
  * **Feature**: This release introduces a new API PutFeedback that allows submitting feedback to Wisdom on content relevance.

# Release (2022-08-14)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.17.0](config/CHANGELOG.md#v1170-2022-08-14)
  * **Feature**: Add alternative mechanism for determning the users `$HOME` or `%USERPROFILE%` location when the environment variables are not present.
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.15.0](service/amp/CHANGELOG.md#v1150-2022-08-14)
  * **Feature**: This release adds log APIs that allow customers to manage logging for their Amazon Managed Service for Prometheus workspaces.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.11.0](service/chimesdkmessaging/CHANGELOG.md#v1110-2022-08-14)
  * **Feature**: The Amazon Chime SDK now supports channels with up to one million participants with elastic channels.
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.17.0](service/ivs/CHANGELOG.md#v1170-2022-08-14)
  * **Feature**: Updates various list api MaxResults ranges
* `github.com/aws/aws-sdk-go-v2/service/personalizeruntime`: [v1.12.0](service/personalizeruntime/CHANGELOG.md#v1120-2022-08-14)
  * **Feature**: This release provides support for promotions in AWS Personalize runtime.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.23.6](service/rds/CHANGELOG.md#v1236-2022-08-14)
  * **Documentation**: Adds support for RDS Custom to DBInstanceClass in ModifyDBInstance

# Release (2022-08-11)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/backupstorage`: [v1.0.0](service/backupstorage/CHANGELOG.md#v100-2022-08-11)
  * **Release**: New AWS service client module
  * **Feature**: This is the first public release of AWS Backup Storage. We are exposing some previously-internal APIs for use by external services. These APIs are not meant to be used directly by customers.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.30.0](service/glue/CHANGELOG.md#v1300-2022-08-11)
  * **Feature**: Add support for Python 3.9 AWS Glue Python Shell jobs
* `github.com/aws/aws-sdk-go-v2/service/privatenetworks`: [v1.0.0](service/privatenetworks/CHANGELOG.md#v100-2022-08-11)
  * **Release**: New AWS service client module
  * **Feature**: This is the initial SDK release for AWS Private 5G. AWS Private 5G is a managed service that makes it easy to deploy, operate, and scale your own private mobile network at your on-premises location.

# Release (2022-08-10)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.16.0](config/CHANGELOG.md#v1160-2022-08-10)
  * **Feature**: Adds support for the following settings in the `~/.aws/credentials` file: `sso_account_id`, `sso_region`, `sso_role_name`, `sso_start_url`, and `ca_bundle`.
* `github.com/aws/aws-sdk-go-v2/service/dlm`: [v1.12.0](service/dlm/CHANGELOG.md#v1120-2022-08-10)
  * **Feature**: This release adds support for excluding specific data (non-boot) volumes from multi-volume snapshot sets created by snapshot lifecycle policies
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.52.0](service/ec2/CHANGELOG.md#v1520-2022-08-10)
  * **Feature**: This release adds support for excluding specific data (non-root) volumes from multi-volume snapshot sets created from instances.

# Release (2022-08-09)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.20.0](service/cloudwatch/CHANGELOG.md#v1200-2022-08-09)
  * **Feature**: Various quota increases related to dimensions and custom metrics
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.18.0](service/location/CHANGELOG.md#v1180-2022-08-09)
  * **Feature**: Amazon Location Service now allows circular geofences in BatchPutGeofence, PutGeofence, and GetGeofence  APIs.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.39.0](service/sagemaker/CHANGELOG.md#v1390-2022-08-09)
  * **Feature**: Amazon SageMaker Automatic Model Tuning now supports specifying multiple alternate EC2 instance types to make tuning jobs more robust when the preferred instance type is not available due to insufficient capacity.
* `github.com/aws/aws-sdk-go-v2/service/sagemakera2iruntime`: [v1.13.0](service/sagemakera2iruntime/CHANGELOG.md#v1130-2022-08-09)
  * **Feature**: Fix bug with parsing ISO-8601 CreationTime in Java SDK in DescribeHumanLoop

# Release (2022-08-08)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.16.9
  * **Bug Fix**: aws/signer/v4: Fixes a panic in SDK's handling of endpoint URLs with ports by correcting how URL path is parsed from opaque URLs. Fixes [#1294](https://github.com/aws/aws-sdk-go-v2/issues/1294).
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.29.0](service/glue/CHANGELOG.md#v1290-2022-08-08)
  * **Feature**: Add an option to run non-urgent or non-time sensitive Glue Jobs on spare capacity
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.14.10](service/identitystore/CHANGELOG.md#v11410-2022-08-08)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.21.0](service/iotwireless/CHANGELOG.md#v1210-2022-08-08)
  * **Feature**: AWS IoT Wireless release support for sidewalk data reliability.
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.17.0](service/pinpoint/CHANGELOG.md#v1170-2022-08-08)
  * **Feature**: Adds support for Advance Quiet Time in Journeys. Adds RefreshOnSegmentUpdate and WaitForQuietTime to JourneyResponse.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.23.2](service/quicksight/CHANGELOG.md#v1232-2022-08-08)
  * **Documentation**: A series of documentation updates to the QuickSight API reference.
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.11.14](service/sso/CHANGELOG.md#v11114-2022-08-08)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.15.2](service/ssoadmin/CHANGELOG.md#v1152-2022-08-08)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.12.12](service/ssooidc/CHANGELOG.md#v11212-2022-08-08)
  * **Documentation**: Documentation updates to reflect service rename - AWS IAM Identity Center (successor to AWS Single Sign-On)

# Release (2022-08-04)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.13.0](service/chimesdkmeetings/CHANGELOG.md#v1130-2022-08-04)
  * **Feature**: Adds support for Tags on Amazon Chime SDK WebRTC sessions
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.24.0](service/configservice/CHANGELOG.md#v1240-2022-08-04)
  * **Feature**: Add resourceType enums for Athena, GlobalAccelerator, Detective and EC2 types
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.21.3](service/databasemigrationservice/CHANGELOG.md#v1213-2022-08-04)
  * **Documentation**: Documentation updates for Database Migration Service (DMS).
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.28.0](service/iot/CHANGELOG.md#v1280-2022-08-04)
  * **Feature**: The release is to support attach a provisioning template to CACert for JITP function,  Customer now doesn't have to hardcode a roleArn and templateBody during register a CACert to enable JITP.

# Release (2022-08-03)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.18.0](service/cognitoidentityprovider/CHANGELOG.md#v1180-2022-08-03)
  * **Feature**: Add a new exception type, ForbiddenException, that is returned when request is not allowed
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.22.0](service/wafv2/CHANGELOG.md#v1220-2022-08-03)
  * **Feature**: You can now associate an AWS WAF web ACL with an Amazon Cognito user pool.

# Release (2022-08-02)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/licensemanagerusersubscriptions`: [v1.0.0](service/licensemanagerusersubscriptions/CHANGELOG.md#v100-2022-08-02)
  * **Release**: New AWS service client module
  * **Feature**: This release supports user based subscription for Microsoft Visual Studio Professional and Enterprise on EC2.
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.21.0](service/personalize/CHANGELOG.md#v1210-2022-08-02)
  * **Feature**: This release adds support for incremental bulk ingestion for the Personalize CreateDatasetImportJob API.

# Release (2022-08-01)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.23.1](service/configservice/CHANGELOG.md#v1231-2022-08-01)
  * **Documentation**: Documentation update for PutConfigRule and PutOrganizationConfigRule
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.22.0](service/workspaces/CHANGELOG.md#v1220-2022-08-01)
  * **Feature**: This release introduces ModifySamlProperties, a new API that allows control of SAML properties associated with a WorkSpaces directory. The DescribeWorkspaceDirectories API will now additionally return SAML properties in its responses.

# Release (2022-07-29)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.51.0](service/ec2/CHANGELOG.md#v1510-2022-07-29)
  * **Feature**: Documentation updates for Amazon EC2.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.24.4](service/fsx/CHANGELOG.md#v1244-2022-07-29)
  * **Documentation**: Documentation updates for Amazon FSx
* `github.com/aws/aws-sdk-go-v2/service/shield`: [v1.17.0](service/shield/CHANGELOG.md#v1170-2022-07-29)
  * **Feature**: AWS Shield Advanced now supports filtering for ListProtections and ListProtectionGroups.

# Release (2022-07-28)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.50.1](service/ec2/CHANGELOG.md#v1501-2022-07-28)
  * **Documentation**: Documentation updates for VM Import/Export.
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.16.0](service/elasticsearchservice/CHANGELOG.md#v1160-2022-07-28)
  * **Feature**: This release adds support for gp3 EBS (Elastic Block Store) storage.
* `github.com/aws/aws-sdk-go-v2/service/lookoutvision`: [v1.14.0](service/lookoutvision/CHANGELOG.md#v1140-2022-07-28)
  * **Feature**: This release introduces support for image segmentation models and updates CPU accelerator options for models hosted on edge devices.
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.10.0](service/opensearch/CHANGELOG.md#v1100-2022-07-28)
  * **Feature**: This release adds support for gp3 EBS (Elastic Block Store) storage.

# Release (2022-07-27)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.20.0](service/auditmanager/CHANGELOG.md#v1200-2022-07-27)
  * **Feature**: This release adds an exceeded quota exception to several APIs. We added a ServiceQuotaExceededException for the following operations: CreateAssessment, CreateControl, CreateAssessmentFramework, and UpdateAssessmentStatus.
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.21.0](service/chime/CHANGELOG.md#v1210-2022-07-27)
  * **Feature**: Chime VoiceConnector will now support ValidateE911Address which will allow customers to prevalidate their addresses included in their SIP invites for emergency calling
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.23.0](service/configservice/CHANGELOG.md#v1230-2022-07-27)
  * **Feature**: This release adds ListConformancePackComplianceScores API to support the new compliance score feature, which provides a percentage of the number of compliant rule-resource combinations in a conformance pack compared to the number of total possible rule-resource combinations in the conformance pack.
* `github.com/aws/aws-sdk-go-v2/service/globalaccelerator`: [v1.14.0](service/globalaccelerator/CHANGELOG.md#v1140-2022-07-27)
  * **Feature**: Global Accelerator now supports dual-stack accelerators, enabling support for IPv4 and IPv6 traffic.
* `github.com/aws/aws-sdk-go-v2/service/marketplacecatalog`: [v1.13.0](service/marketplacecatalog/CHANGELOG.md#v1130-2022-07-27)
  * **Feature**: The SDK for the StartChangeSet API will now automatically set and use an idempotency token in the ClientRequestToken request parameter if the customer does not provide it.
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.17.0](service/polly/CHANGELOG.md#v1170-2022-07-27)
  * **Feature**: Amazon Polly adds new English and Hindi voice - Kajal. Kajal is available as Neural voice only.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.27.5](service/ssm/CHANGELOG.md#v1275-2022-07-27)
  * **Documentation**: Adding doc updates for OpsCenter support in Service Setting actions.
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.21.0](service/workspaces/CHANGELOG.md#v1210-2022-07-27)
  * **Feature**: Added CreateWorkspaceImage API to create a new WorkSpace image from an existing WorkSpace.

# Release (2022-07-26)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.15.0](service/appsync/CHANGELOG.md#v1150-2022-07-26)
  * **Feature**: Adds support for a new API to evaluate mapping templates with mock data, allowing you to remotely unit test your AppSync resolvers and functions.
* `github.com/aws/aws-sdk-go-v2/service/detective`: [v1.16.0](service/detective/CHANGELOG.md#v1160-2022-07-26)
  * **Feature**: Added the ability to get data source package information for the behavior graph. Graph administrators can now start (or stop) optional datasources on the behavior graph.
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.15.0](service/guardduty/CHANGELOG.md#v1150-2022-07-26)
  * **Feature**: Amazon GuardDuty introduces a new Malware Protection feature that triggers malware scan on selected EC2 instance resources, after the service detects a potentially malicious activity.
* `github.com/aws/aws-sdk-go-v2/service/lookoutvision`: [v1.13.0](service/lookoutvision/CHANGELOG.md#v1130-2022-07-26)
  * **Feature**: This release introduces support for the automatic scaling of inference units used by Amazon Lookout for Vision models.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.22.0](service/macie2/CHANGELOG.md#v1220-2022-07-26)
  * **Feature**: This release adds support for retrieving (revealing) sample occurrences of sensitive data that Amazon Macie detects and reports in findings.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.23.1](service/rds/CHANGELOG.md#v1231-2022-07-26)
  * **Documentation**: Adds support for using RDS Proxies with RDS for MariaDB databases.
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.19.0](service/rekognition/CHANGELOG.md#v1190-2022-07-26)
  * **Feature**: This release introduces support for the automatic scaling of inference units used by Amazon Rekognition Custom Labels models.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.22.3](service/securityhub/CHANGELOG.md#v1223-2022-07-26)
  * **Documentation**: Documentation updates for AWS Security Hub
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.21.0](service/transfer/CHANGELOG.md#v1210-2022-07-26)
  * **Feature**: AWS Transfer Family now supports Applicability Statement 2 (AS2), a network protocol used for the secure and reliable transfer of critical Business-to-Business (B2B) data over the public internet using HTTP/HTTPS as the transport mechanism.

# Release (2022-07-25)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.23.6](service/autoscaling/CHANGELOG.md#v1236-2022-07-25)
  * **Documentation**: Documentation update for Amazon EC2 Auto Scaling.

# Release (2022-07-22)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/account`: [v1.7.0](service/account/CHANGELOG.md#v170-2022-07-22)
  * **Feature**: This release enables customers to manage the primary contact information for their AWS accounts. For more information, see https://docs.aws.amazon.com/accounts/latest/reference/API_Operations.html
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.50.0](service/ec2/CHANGELOG.md#v1500-2022-07-22)
  * **Feature**: Added support for EC2 M1 Mac instances. For more information, please visit aws.amazon.com/mac.
* `github.com/aws/aws-sdk-go-v2/service/iotdeviceadvisor`: [v1.15.0](service/iotdeviceadvisor/CHANGELOG.md#v1150-2022-07-22)
  * **Feature**: Added new service feature (Early access only) - Long Duration Test, where customers can test the IoT device to observe how it behaves when the device is in operation for longer period.
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.22.0](service/medialive/CHANGELOG.md#v1220-2022-07-22)
  * **Feature**: Link devices now support remote rebooting. Link devices now support maintenance windows. Maintenance windows allow a Link device to install software updates without stopping the MediaLive channel. The channel will experience a brief loss of input from the device while updates are installed.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.23.0](service/rds/CHANGELOG.md#v1230-2022-07-22)
  * **Feature**: This release adds the "ModifyActivityStream" API with support for audit policy state locking and unlocking.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.21.0](service/transcribe/CHANGELOG.md#v1210-2022-07-22)
  * **Feature**: Remove unsupported language codes for StartTranscriptionJob and update VocabularyFileUri for UpdateMedicalVocabulary

# Release (2022-07-21)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.18.0](service/athena/CHANGELOG.md#v1180-2022-07-21)
  * **Feature**: This feature allows customers to retrieve runtime statistics for completed queries
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.19.0](service/cloudwatch/CHANGELOG.md#v1190-2022-07-21)
  * **Feature**: Adding support for the suppression of Composite Alarm actions
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.21.1](service/databasemigrationservice/CHANGELOG.md#v1211-2022-07-21)
  * **Documentation**: Documentation updates for Database Migration Service (DMS).
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.19.0](service/docdb/CHANGELOG.md#v1190-2022-07-21)
  * **Feature**: Enable copy-on-write restore type
* `github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect`: [v1.14.0](service/ec2instanceconnect/CHANGELOG.md#v1140-2022-07-21)
  * **Feature**: This release includes a new exception type "EC2InstanceUnavailableException" for SendSSHPublicKey and SendSerialConsoleSSHPublicKey APIs.
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.20.0](service/frauddetector/CHANGELOG.md#v1200-2022-07-21)
  * **Feature**: The release introduces Account Takeover Insights (ATI) model. The ATI model detects fraud relating to account takeover. This release also adds support for new variable types: ARE_CREDENTIALS_VALID and SESSION_ID and adds new structures to Model Version APIs.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.23.0](service/iotsitewise/CHANGELOG.md#v1230-2022-07-21)
  * **Feature**: Added asynchronous API to ingest bulk historical and current data into IoT SiteWise.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.31.0](service/kendra/CHANGELOG.md#v1310-2022-07-21)
  * **Feature**: Amazon Kendra now provides Oauth2 support for SharePoint Online. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-sharepoint.html
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.18.0](service/networkfirewall/CHANGELOG.md#v1180-2022-07-21)
  * **Feature**: Network Firewall now supports referencing dynamic IP sets from stateful rule groups, for IP sets stored in Amazon VPC prefix lists.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.22.1](service/rds/CHANGELOG.md#v1221-2022-07-21)
  * **Documentation**: Adds support for creating an RDS Proxy for an RDS for MariaDB database.

# Release (2022-07-20)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.17.11](service/acmpca/CHANGELOG.md#v11711-2022-07-20)
  * **Documentation**: AWS Certificate Manager (ACM) Private Certificate Authority (PCA) documentation updates
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.27.0](service/iot/CHANGELOG.md#v1270-2022-07-20)
  * **Feature**: GA release the ability to enable/disable IoT Fleet Indexing for Device Defender and Named Shadow information, and search them through IoT Fleet Indexing APIs. This includes Named Shadow Selection as a part of the UpdateIndexingConfiguration API.

# Release (2022-07-19)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.18.0](service/devopsguru/CHANGELOG.md#v1180-2022-07-19)
  * **Feature**: Added new APIs for log anomaly detection feature.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.28.1](service/glue/CHANGELOG.md#v1281-2022-07-19)
  * **Documentation**: Documentation updates for AWS Glue Job Timeout and Autoscaling
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.38.0](service/sagemaker/CHANGELOG.md#v1380-2022-07-19)
  * **Feature**: Fixed an issue with cross account QueryLineage
* `github.com/aws/aws-sdk-go-v2/service/sagemakeredge`: [v1.12.0](service/sagemakeredge/CHANGELOG.md#v1120-2022-07-19)
  * **Feature**: Amazon SageMaker Edge Manager provides lightweight model deployment feature to deploy machine learning models on requested devices.
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.20.0](service/workspaces/CHANGELOG.md#v1200-2022-07-19)
  * **Feature**: Increased the character limit of the login message from 850 to 2000 characters.

# Release (2022-07-18)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/applicationdiscoveryservice`: [v1.14.0](service/applicationdiscoveryservice/CHANGELOG.md#v1140-2022-07-18)
  * **Feature**: Add AWS Agentless Collector details to the GetDiscoverySummary API response
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.49.1](service/ec2/CHANGELOG.md#v1491-2022-07-18)
  * **Documentation**: Documentation updates for Amazon EC2.
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.22.0](service/elasticache/CHANGELOG.md#v1220-2022-07-18)
  * **Feature**: Adding AutoMinorVersionUpgrade in the DescribeReplicationGroups API
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.18.0](service/kms/CHANGELOG.md#v1180-2022-07-18)
  * **Feature**: Added support for the SM2 KeySpec in China Partition Regions
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.17.0](service/mediapackage/CHANGELOG.md#v1170-2022-07-18)
  * **Feature**: This release adds "IncludeIframeOnlyStream" for Dash endpoints and increases the number of supported video and audio encryption presets for Speke v2
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.37.0](service/sagemaker/CHANGELOG.md#v1370-2022-07-18)
  * **Feature**: Amazon SageMaker Edge Manager provides lightweight model deployment feature to deploy machine learning models on requested devices.
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.15.0](service/ssoadmin/CHANGELOG.md#v1150-2022-07-18)
  * **Feature**: AWS SSO now supports attaching customer managed policies and a permissions boundary to your permission sets. This release adds new API operations to manage and view the customer managed policies and the permissions boundary for a given permission set.

# Release (2022-07-15)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.18.3](service/datasync/CHANGELOG.md#v1183-2022-07-15)
  * **Documentation**: Documentation updates for AWS DataSync regarding configuring Amazon FSx for ONTAP location security groups and SMB user permissions.
* `github.com/aws/aws-sdk-go-v2/service/drs`: [v1.7.0](service/drs/CHANGELOG.md#v170-2022-07-15)
  * **Feature**: Changed existing APIs to allow choosing a dynamic volume type for replicating volumes, to reduce costs for customers.
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.8.0](service/evidently/CHANGELOG.md#v180-2022-07-15)
  * **Feature**: This release adds support for the new segmentation feature.
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.21.0](service/wafv2/CHANGELOG.md#v1210-2022-07-15)
  * **Feature**: This SDK release provide customers ability to add sensitivity level for WAF SQLI Match Statements.

# Release (2022-07-14)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.17.0](service/athena/CHANGELOG.md#v1170-2022-07-14)
  * **Feature**: This release updates data types that contain either QueryExecutionId, NamedQueryId or ExpectedBucketOwner. Ids must be between 1 and 128 characters and contain only non-whitespace characters. ExpectedBucketOwner must be 12-digit string.
* `github.com/aws/aws-sdk-go-v2/service/codeartifact`: [v1.13.0](service/codeartifact/CHANGELOG.md#v1130-2022-07-14)
  * **Feature**: This release introduces Package Origin Controls, a mechanism used to counteract Dependency Confusion attacks. Adds two new APIs, PutPackageOriginConfiguration and DescribePackage, and updates the ListPackage, DescribePackageVersion and ListPackageVersion APIs in support of the feature.
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.22.0](service/configservice/CHANGELOG.md#v1220-2022-07-14)
  * **Feature**: Update ResourceType enum with values for Route53Resolver, Batch, DMS, Workspaces, Stepfunctions, SageMaker, ElasticLoadBalancingV2, MSK types
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.49.0](service/ec2/CHANGELOG.md#v1490-2022-07-14)
  * **Feature**: This release adds flow logs for Transit Gateway to  allow customers to gain deeper visibility and insights into network traffic through their Transit Gateways.
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.18.0](service/fms/CHANGELOG.md#v1180-2022-07-14)
  * **Feature**: Adds support for strict ordering in stateful rule groups in Network Firewall policies.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.28.0](service/glue/CHANGELOG.md#v1280-2022-07-14)
  * **Feature**: This release adds an additional worker type for Glue Streaming jobs.
* `github.com/aws/aws-sdk-go-v2/service/inspector2`: [v1.7.0](service/inspector2/CHANGELOG.md#v170-2022-07-14)
  * **Feature**: This release adds support for Inspector V2 scan configurations through the get and update configuration APIs. Currently this allows configuring ECR automated re-scan duration to lifetime or 180 days or 30 days.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.30.0](service/kendra/CHANGELOG.md#v1300-2022-07-14)
  * **Feature**: This release adds AccessControlConfigurations which allow you to redefine your document level access control without the need for content re-indexing.
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.13.0](service/nimble/CHANGELOG.md#v1130-2022-07-14)
  * **Feature**: Amazon Nimble Studio adds support for IAM-based access to AWS resources for Nimble Studio components and custom studio components. Studio Component scripts use these roles on Nimble Studio workstation to mount filesystems, access S3 buckets, or other configured resources in the Studio's AWS account
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.22.0](service/outposts/CHANGELOG.md#v1220-2022-07-14)
  * **Feature**: This release adds the ShipmentInformation and AssetInformationList fields to the GetOrder API response.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.36.0](service/sagemaker/CHANGELOG.md#v1360-2022-07-14)
  * **Feature**: This release adds support for G5, P4d, and C6i instance types in Amazon SageMaker Inference and increases the number of hyperparameters that can be searched from 20 to 30 in Amazon SageMaker Automatic Model Tuning

# Release (2022-07-13)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appconfig`: [v1.13.0](service/appconfig/CHANGELOG.md#v1130-2022-07-13)
  * **Feature**: Adding Create, Get, Update, Delete, and List APIs for new two new resources: Extensions and ExtensionAssociations.

# Release (2022-07-12)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.14.0](service/networkmanager/CHANGELOG.md#v1140-2022-07-12)
  * **Feature**: This release adds general availability API support for AWS Cloud WAN.

# Release (2022-07-11)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.48.0](service/ec2/CHANGELOG.md#v1480-2022-07-11)
  * **Feature**: Build, manage, and monitor a unified global network that connects resources running across your cloud and on-premises environments using the AWS Cloud WAN APIs.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.26.0](service/redshift/CHANGELOG.md#v1260-2022-07-11)
  * **Feature**: This release adds a new --snapshot-arn field for describe-cluster-snapshots, describe-node-configuration-options, restore-from-cluster-snapshot, authorize-snapshot-acsess, and revoke-snapshot-acsess APIs. It allows customers to give a Redshift snapshot ARN or a Redshift Serverless ARN as input.
* `github.com/aws/aws-sdk-go-v2/service/redshiftserverless`: [v1.2.2](service/redshiftserverless/CHANGELOG.md#v122-2022-07-11)
  * **Documentation**: Removed prerelease language for GA launch.

# Release (2022-07-08)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.17.0](service/backup/CHANGELOG.md#v1170-2022-07-08)
  * **Feature**: This release adds support for authentication using IAM user identity instead of passed IAM role, identified by excluding the IamRoleArn field in the StartRestoreJob API. This feature applies to only resource clients with a destructive restore nature (e.g. SAP HANA).

# Release (2022-07-07)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.12.0](service/chimesdkmeetings/CHANGELOG.md#v1120-2022-07-07)
  * **Feature**: Adds support for AppKeys and TenantIds in Amazon Chime SDK WebRTC sessions
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.21.0](service/databasemigrationservice/CHANGELOG.md#v1210-2022-07-07)
  * **Feature**: New api to migrate event subscriptions to event bridge rules
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.26.0](service/iot/CHANGELOG.md#v1260-2022-07-07)
  * **Feature**: This release adds support to register a CA certificate without having to provide a verification certificate. This also allows multiple AWS accounts to register the same CA in the same region.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.20.0](service/iotwireless/CHANGELOG.md#v1200-2022-07-07)
  * **Feature**: Adds 5 APIs: PutPositionConfiguration, GetPositionConfiguration, ListPositionConfigurations, UpdatePosition, GetPosition for the new Positioning Service feature which enables customers to configure solvers to calculate position of LoRaWAN devices, or specify position of LoRaWAN devices & gateways.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.35.0](service/sagemaker/CHANGELOG.md#v1350-2022-07-07)
  * **Feature**: Heterogeneous clusters: the ability to launch training jobs with multiple instance types. This enables running component of the training job on the instance type that is most suitable for it. e.g. doing data processing and augmentation on CPU instances and neural network training on GPU instances

# Release (2022-07-06)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.22.0](service/cloudformation/CHANGELOG.md#v1220-2022-07-06)
  * **Feature**: My AWS Service (placeholder) - Add a new feature Account-level Targeting for StackSet operation
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.16.0](service/synthetics/CHANGELOG.md#v1160-2022-07-06)
  * **Feature**: This release introduces Group feature, which enables users to group cross-region canaries.

# Release (2022-07-05)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.21.5](service/configservice/CHANGELOG.md#v1215-2022-07-05)
  * **Documentation**: Updating documentation service limits
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.21.0](service/lexmodelsv2/CHANGELOG.md#v1210-2022-07-05)
  * **Feature**: This release introduces additional optional parameters "messageSelectionStrategy" to PromptSpecification, which enables the users to configure the bot to play messages in orderly manner.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.23.0](service/quicksight/CHANGELOG.md#v1230-2022-07-05)
  * **Feature**: This release allows customers to programmatically create QuickSight accounts with Enterprise and Enterprise + Q editions. It also releases allowlisting domains for embedding QuickSight dashboards at runtime through the embedding APIs.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.22.0](service/rds/CHANGELOG.md#v1220-2022-07-05)
  * **Feature**: Adds waiters support for DBCluster.
* `github.com/aws/aws-sdk-go-v2/service/rolesanywhere`: [v1.0.0](service/rolesanywhere/CHANGELOG.md#v100-2022-07-05)
  * **Release**: New AWS service client module
  * **Feature**: IAM Roles Anywhere allows your workloads such as servers, containers, and applications to obtain temporary AWS credentials and use the same IAM roles and policies that you have configured for your AWS workloads to access AWS resources.
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.19.0](service/sqs/CHANGELOG.md#v1190-2022-07-05)
  * **Feature**: Adds support for the SQS client to automatically validate message checksums for SendMessage, SendMessageBatch, and ReceiveMessage. A DisableMessageChecksumValidation parameter has been added to the Options struct for SQS package. Setting this to true will disable the checksum validation. This can be set when creating a client, or per operation call.
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.15.0](service/ssmincidents/CHANGELOG.md#v1150-2022-07-05)
  * **Feature**: Adds support for tagging incident-record on creation by providing incident tags in the template within a response-plan.

# Release (2022-07-01)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.20.0](service/databasemigrationservice/CHANGELOG.md#v1200-2022-07-01)
  * **Feature**: Added new features for AWS DMS version 3.4.7 that includes new endpoint settings for S3, OpenSearch, Postgres, SQLServer and Oracle.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.21.5](service/rds/CHANGELOG.md#v1215-2022-07-01)
  * **Documentation**: Adds support for additional retention periods to Performance Insights.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.27.0](service/s3/CHANGELOG.md#v1270-2022-07-01)
  * **Feature**: Add presign support for HeadBucket, DeleteObject, and DeleteBucket. Fixes [#1076](https://github.com/aws/aws-sdk-go-v2/issues/1076).

# Release (2022-06-30)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.16.0](service/athena/CHANGELOG.md#v1160-2022-06-30)
  * **Feature**: This feature introduces the API support for Athena's parameterized query and BatchGetPreparedStatement API.
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.18.0](service/customerprofiles/CHANGELOG.md#v1180-2022-06-30)
  * **Feature**: This release adds the optional MinAllowedConfidenceScoreForMerging parameter to the CreateDomain, UpdateDomain, and GetAutoMergingPreview APIs in Customer Profiles. This parameter is used as a threshold to influence the profile auto-merging step of the Identity Resolution process.
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.20.0](service/emr/CHANGELOG.md#v1200-2022-06-30)
  * **Feature**: This release adds support for the ExecutionRoleArn parameter in the AddJobFlowSteps and DescribeStep APIs. Customers can use ExecutionRoleArn to specify the IAM role used for each job they submit using the AddJobFlowSteps API.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.27.0](service/glue/CHANGELOG.md#v1270-2022-06-30)
  * **Feature**: This release adds tag as an input of CreateDatabase
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.29.0](service/kendra/CHANGELOG.md#v1290-2022-06-30)
  * **Feature**: Amazon Kendra now provides a data source connector for alfresco
* `github.com/aws/aws-sdk-go-v2/service/mwaa`: [v1.13.0](service/mwaa/CHANGELOG.md#v1130-2022-06-30)
  * **Feature**: Documentation updates for Amazon Managed Workflows for Apache Airflow.
* `github.com/aws/aws-sdk-go-v2/service/pricing`: [v1.16.0](service/pricing/CHANGELOG.md#v1160-2022-06-30)
  * **Feature**: Documentation update for GetProducts Response.
* `github.com/aws/aws-sdk-go-v2/service/wellarchitected`: [v1.16.0](service/wellarchitected/CHANGELOG.md#v1160-2022-06-30)
  * **Feature**: Added support for UpdateGlobalSettings API. Added status filter to ListWorkloadShares and ListLensShares.
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.16.0](service/workmail/CHANGELOG.md#v1160-2022-06-30)
  * **Feature**: This release adds support for managing user availability configurations in Amazon WorkMail.

# Release (2022-06-29)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.16.6
  * **Bug Fix**: Fix aws/signer/v4 to not double sign Content-Length header. Fixes [#1728](https://github.com/aws/aws-sdk-go-v2/issues/1728). Thanks to @matelang for creating the issue and PR.
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.17.0](service/appstream/CHANGELOG.md#v1170-2022-06-29)
  * **Feature**: Includes support for StreamingExperienceSettings in CreateStack and UpdateStack APIs
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.18.7](service/elasticloadbalancingv2/CHANGELOG.md#v1187-2022-06-29)
  * **Documentation**: This release adds two attributes for ALB. One, helps to preserve the host header and the other helps to modify, preserve, or remove the X-Forwarded-For header in the HTTP request.
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.19.0](service/emr/CHANGELOG.md#v1190-2022-06-29)
  * **Feature**: This release introduces additional optional parameter "Throughput" to VolumeSpecification to enable user to configure throughput for gp3 ebs volumes.
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.21.0](service/medialive/CHANGELOG.md#v1210-2022-06-29)
  * **Feature**: This release adds support for automatic renewal of MediaLive reservations at the end of each reservation term. Automatic renewal is optional. This release also adds support for labelling accessibility-focused audio and caption tracks in HLS outputs.
* `github.com/aws/aws-sdk-go-v2/service/redshiftserverless`: [v1.2.0](service/redshiftserverless/CHANGELOG.md#v120-2022-06-29)
  * **Feature**: Add new API operations for Amazon Redshift Serverless, a new way of using Amazon Redshift without needing to manually manage provisioned clusters. The new operations let you interact with Redshift Serverless resources, such as create snapshots, list VPC endpoints, delete resource policies, and more.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.34.0](service/sagemaker/CHANGELOG.md#v1340-2022-06-29)
  * **Feature**: This release adds: UpdateFeatureGroup, UpdateFeatureMetadata, DescribeFeatureMetadata APIs; FeatureMetadata type in Search API; LastModifiedTime, LastUpdateStatus, OnlineStoreTotalSizeBytes in DescribeFeatureGroup API.
* `github.com/aws/aws-sdk-go-v2/service/translate`: [v1.14.0](service/translate/CHANGELOG.md#v1140-2022-06-29)
  * **Feature**: Added ListLanguages API which can be used to list the languages supported by Translate.

# Release (2022-06-28)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.18.0](service/datasync/CHANGELOG.md#v1180-2022-06-28)
  * **Feature**: AWS DataSync now supports Amazon FSx for NetApp ONTAP locations.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.47.0](service/ec2/CHANGELOG.md#v1470-2022-06-28)
  * **Feature**: This release adds a new spread placement group to EC2 Placement Groups: host level spread, which spread instances between physical hosts, available to Outpost customers only. CreatePlacementGroup and DescribePlacementGroups APIs were updated with a new parameter: SpreadLevel to support this feature.
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.12.0](service/finspacedata/CHANGELOG.md#v1120-2022-06-28)
  * **Feature**: Release new API GetExternalDataViewAccessDetails
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.16.0](service/polly/CHANGELOG.md#v1160-2022-06-28)
  * **Feature**: Add 4 new neural voices - Pedro (es-US), Liam (fr-CA), Daniel (de-DE) and Arthur (en-GB).

# Release (2022-06-24.2)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/emrcontainers`: [v1.13.7](service/emrcontainers/CHANGELOG.md#v1137-2022-06-242)
  * **Bug Fix**: Fixes bug with incorrect modeled timestamp format

# Release (2022-06-23)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.14.0](service/lookoutequipment/CHANGELOG.md#v1140-2022-06-23)
  * **Feature**: This release adds visualizations to the scheduled inference results. Users will be able to see interference results, including diagnostic results from their running inference schedulers.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.25.1](service/mediaconvert/CHANGELOG.md#v1251-2022-06-23)
  * **Documentation**: AWS Elemental MediaConvert SDK has released support for automatic DolbyVision metadata generation when converting HDR10 to DolbyVision.
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.15.0](service/mgn/CHANGELOG.md#v1150-2022-06-23)
  * **Feature**: New and modified APIs for the Post-Migration Framework
* `github.com/aws/aws-sdk-go-v2/service/migrationhubrefactorspaces`: [v1.6.0](service/migrationhubrefactorspaces/CHANGELOG.md#v160-2022-06-23)
  * **Feature**: This release adds the new API UpdateRoute that allows route to be updated to ACTIVE/INACTIVE state. In addition, CreateRoute API will now allow users to create route in ACTIVE/INACTIVE state.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.33.0](service/sagemaker/CHANGELOG.md#v1330-2022-06-23)
  * **Feature**: SageMaker Ground Truth now supports Virtual Private Cloud. Customers can launch labeling jobs and access to their private workforce in VPC mode.

# Release (2022-06-22)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.15.8](service/apigateway/CHANGELOG.md#v1158-2022-06-22)
  * **Documentation**: Documentation updates for Amazon API Gateway
* `github.com/aws/aws-sdk-go-v2/service/pricing`: [v1.15.0](service/pricing/CHANGELOG.md#v1150-2022-06-22)
  * **Feature**: This release introduces 1 update to the GetProducts API. The serviceCode attribute is now required when you use the GetProductsRequest.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.20.0](service/transfer/CHANGELOG.md#v1200-2022-06-22)
  * **Feature**: Until today, the service supported only RSA host keys and user keys. Now with this launch, Transfer Family has expanded the support for ECDSA and ED25519 host keys and user keys, enabling customers to support a broader set of clients by choosing RSA, ECDSA, and ED25519 host and user keys.

# Release (2022-06-21)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.46.0](service/ec2/CHANGELOG.md#v1460-2022-06-21)
  * **Feature**: This release adds support for Private IP VPNs, a new feature allowing S2S VPN connections to use private ip addresses as the tunnel outside ip address over Direct Connect as transport.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.18.9](service/ecs/CHANGELOG.md#v1189-2022-06-21)
  * **Documentation**: Amazon ECS UpdateService now supports the following parameters: PlacementStrategies, PlacementConstraints and CapacityProviderStrategy.
* `github.com/aws/aws-sdk-go-v2/service/wellarchitected`: [v1.15.0](service/wellarchitected/CHANGELOG.md#v1150-2022-06-21)
  * **Feature**: Adds support for lens tagging, Adds support for multiple helpful-resource urls and multiple improvement-plan urls.

# Release (2022-06-20)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/directoryservice`: [v1.14.0](service/directoryservice/CHANGELOG.md#v1140-2022-06-20)
  * **Feature**: This release adds support for describing and updating AWS Managed Microsoft AD settings
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.17.7](service/kafka/CHANGELOG.md#v1177-2022-06-20)
  * **Documentation**: Documentation updates to use Az Id during cluster creation.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.21.0](service/outposts/CHANGELOG.md#v1210-2022-06-20)
  * **Feature**: This release adds the AssetLocation structure to the ListAssets response. AssetLocation includes the RackElevation for an Asset.

# Release (2022-06-17)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.27.0](service/connect/CHANGELOG.md#v1270-2022-06-17)
  * **Feature**: This release updates these APIs: UpdateInstanceAttribute, DescribeInstanceAttribute and ListInstanceAttributes. You can use it to programmatically enable/disable High volume outbound communications using attribute type HIGH_VOLUME_OUTBOUND on the specified Amazon Connect instance.
* `github.com/aws/aws-sdk-go-v2/service/connectcampaigns`: [v1.0.0](service/connectcampaigns/CHANGELOG.md#v100-2022-06-17)
  * **Release**: New AWS service client module
  * **Feature**: Added Amazon Connect high volume outbound communications SDK.
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.15.7](service/dynamodb/CHANGELOG.md#v1157-2022-06-17)
  * **Documentation**: Doc only update for DynamoDB service
* `github.com/aws/aws-sdk-go-v2/service/dynamodbstreams`: [v1.13.7](service/dynamodbstreams/CHANGELOG.md#v1137-2022-06-17)
  * **Documentation**: Doc only update for DynamoDB service

# Release (2022-06-16)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/redshiftdata`: [v1.16.0](service/redshiftdata/CHANGELOG.md#v1160-2022-06-16)
  * **Feature**: This release adds a new --workgroup-name field to operations that connect to an endpoint. Customers can now execute queries against their serverless workgroups.
* `github.com/aws/aws-sdk-go-v2/service/redshiftserverless`: [v1.1.0](service/redshiftserverless/CHANGELOG.md#v110-2022-06-16)
  * **Feature**: Add new API operations for Amazon Redshift Serverless, a new way of using Amazon Redshift without needing to manually manage provisioned clusters. The new operations let you interact with Redshift Serverless resources, such as create snapshots, list VPC endpoints, delete resource policies, and more.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.11](service/secretsmanager/CHANGELOG.md#v11511-2022-06-16)
  * **Documentation**: Documentation updates for Secrets Manager
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.22.0](service/securityhub/CHANGELOG.md#v1220-2022-06-16)
  * **Feature**: Added Threats field for security findings. Added new resource details for ECS Container, ECS Task, RDS SecurityGroup, Kinesis Stream, EC2 TransitGateway, EFS AccessPoint, CloudFormation Stack, CloudWatch Alarm, VPC Peering Connection and WAF Rules

# Release (2022-06-15)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.11.0](service/finspacedata/CHANGELOG.md#v1110-2022-06-15)
  * **Feature**: This release adds a new set of APIs, GetPermissionGroup, DisassociateUserFromPermissionGroup, AssociateUserToPermissionGroup, ListPermissionGroupsByUser, ListUsersByPermissionGroup.
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.14.0](service/guardduty/CHANGELOG.md#v1140-2022-06-15)
  * **Feature**: Adds finding fields available from GuardDuty Console. Adds FreeTrial related operations. Deprecates the use of various APIs related to Master Accounts and Replace them with Administrator Accounts.
* `github.com/aws/aws-sdk-go-v2/service/servicecatalogappregistry`: [v1.13.0](service/servicecatalogappregistry/CHANGELOG.md#v1130-2022-06-15)
  * **Feature**: This release adds a new API ListAttributeGroupsForApplication that returns associated attribute groups of an application. In addition, the UpdateApplication and UpdateAttributeGroup APIs will not allow users to update the 'Name' attribute.
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.19.0](service/workspaces/CHANGELOG.md#v1190-2022-06-15)
  * **Feature**: Added new field "reason" to OperationNotSupportedException. Receiving this exception in the DeregisterWorkspaceDirectory API will now return a reason giving more context on the failure.

# Release (2022-06-14)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/budgets`: [v1.13.0](service/budgets/CHANGELOG.md#v1130-2022-06-14)
  * **Feature**: Add a budgets ThrottlingException. Update the CostFilters value pattern.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.16.0](service/lookoutmetrics/CHANGELOG.md#v1160-2022-06-14)
  * **Feature**: Adding filters to Alert and adding new UpdateAlert API.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.25.0](service/mediaconvert/CHANGELOG.md#v1250-2022-06-14)
  * **Feature**: AWS Elemental MediaConvert SDK has added support for rules that constrain Automatic-ABR rendition selection when generating ABR package ladders.

# Release (2022-06-13)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.20.0](service/outposts/CHANGELOG.md#v1200-2022-06-13)
  * **Feature**: This release adds API operations AWS uses to install Outpost servers.

# Release (2022-06-10)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.19.7](service/frauddetector/CHANGELOG.md#v1197-2022-06-10)
  * **Documentation**: Documentation updates for Amazon Fraud Detector (AWSHawksNest)

# Release (2022-06-09)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.11.0](service/chimesdkmeetings/CHANGELOG.md#v1110-2022-06-09)
  * **Feature**: Adds support for live transcription in AWS GovCloud (US) Regions.

# Release (2022-06-08)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.19.0](service/databasemigrationservice/CHANGELOG.md#v1190-2022-06-08)
  * **Feature**: This release adds DMS Fleet Advisor APIs and exposes functionality for DMS Fleet Advisor. It adds functionality to create and modify fleet advisor instances, and to collect and analyze information about the local data infrastructure.
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.18.7](service/iam/CHANGELOG.md#v1187-2022-06-08)
  * **Documentation**: Documentation updates for AWS Identity and Access Management (IAM).
* `github.com/aws/aws-sdk-go-v2/service/m2`: [v1.0.0](service/m2/CHANGELOG.md#v100-2022-06-08)
  * **Release**: New AWS service client module
  * **Feature**: AWS Mainframe Modernization service is a managed mainframe service and set of tools for planning, migrating, modernizing, and running mainframe workloads on AWS
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.17.0](service/neptune/CHANGELOG.md#v1170-2022-06-08)
  * **Feature**: This release adds support for Neptune to be configured as a global database, with a primary DB cluster in one region, and up to five secondary DB clusters in other regions.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.25.0](service/redshift/CHANGELOG.md#v1250-2022-06-08)
  * **Feature**: Adds new API GetClusterCredentialsWithIAM to return temporary credentials.
* `github.com/aws/aws-sdk-go-v2/service/redshiftserverless`: [v1.0.0](service/redshiftserverless/CHANGELOG.md#v100-2022-06-08)
  * **Release**: New AWS service client module
  * **Feature**: Add new API operations for Amazon Redshift Serverless, a new way of using Amazon Redshift without needing to manually manage provisioned clusters. The new operations let you interact with Redshift Serverless resources, such as create snapshots, list VPC endpoints, delete resource policies, and more.

# Release (2022-06-07)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.19.0](service/auditmanager/CHANGELOG.md#v1190-2022-06-07)
  * **Feature**: This release introduces 2 updates to the Audit Manager API. The roleType and roleArn attributes are now required when you use the CreateAssessment or UpdateAssessment operation. We also added a throttling exception to the RegisterAccount API operation.
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.19.0](service/costexplorer/CHANGELOG.md#v1190-2022-06-07)
  * **Feature**: Added two new APIs to support cost allocation tags operations: ListCostAllocationTags, UpdateCostAllocationTagsStatus.

# Release (2022-06-06)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.10.0](service/chimesdkmessaging/CHANGELOG.md#v1100-2022-06-06)
  * **Feature**: This release adds support for searching channels by members via the SearchChannels API, removes required restrictions for Name and Mode in UpdateChannel API and enhances CreateChannel API by exposing member and moderator list as well as channel id as optional parameters.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.26.0](service/connect/CHANGELOG.md#v1260-2022-06-06)
  * **Feature**: This release adds a new API, GetCurrentUserData, which returns real-time details about users' current activity.

# Release (2022-06-02)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.16.0](service/applicationinsights/CHANGELOG.md#v1160-2022-06-02)
  * **Feature**: Provide Account Level onboarding support through CFN/CLI
* `github.com/aws/aws-sdk-go-v2/service/codeartifact`: [v1.12.6](service/codeartifact/CHANGELOG.md#v1126-2022-06-02)
  * **Documentation**: Documentation updates for CodeArtifact
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.25.0](service/connect/CHANGELOG.md#v1250-2022-06-02)
  * **Feature**: This release adds the following features: 1) New APIs to manage (create, list, update) task template resources, 2) Updates to startTaskContact API to support task templates, and 3) new TransferContact API to programmatically transfer in-progress tasks via a contact flow.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.28.0](service/kendra/CHANGELOG.md#v1280-2022-06-02)
  * **Feature**: Amazon Kendra now provides a data source connector for GitHub. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-github.html
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.14.0](service/proton/CHANGELOG.md#v1140-2022-06-02)
  * **Feature**: Add new "Components" API to enable users to Create, Delete and Update AWS Proton components.
* `github.com/aws/aws-sdk-go-v2/service/voiceid`: [v1.10.0](service/voiceid/CHANGELOG.md#v1100-2022-06-02)
  * **Feature**: Added a new attribute ServerSideEncryptionUpdateDetails to Domain and DomainSummary.

# Release (2022-06-01)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/backupgateway`: [v1.6.0](service/backupgateway/CHANGELOG.md#v160-2022-06-01)
  * **Feature**: Adds GetGateway and UpdateGatewaySoftwareNow API and adds hypervisor name to UpdateHypervisor API
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.10.0](service/chimesdkmeetings/CHANGELOG.md#v1100-2022-06-01)
  * **Feature**: Adds support for centrally controlling each participant's ability to send and receive audio, video and screen share within a WebRTC session.  Attendee capabilities can be specified when the attendee is created and updated during the session with the new BatchUpdateAttendeeCapabilitiesExcept API.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.22.0](service/forecast/CHANGELOG.md#v1220-2022-06-01)
  * **Feature**: Added Format field to Import and Export APIs in Amazon Forecast. Added TimeSeriesSelector to Create Forecast API.
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.21.0](service/route53/CHANGELOG.md#v1210-2022-06-01)
  * **Feature**: Add new APIs to support Route 53 IP Based Routing

# Release (2022-05-31)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.17.0](service/cognitoidentityprovider/CHANGELOG.md#v1170-2022-05-31)
  * **Feature**: Amazon Cognito now supports IP Address propagation for all unauthenticated APIs (e.g. SignUp, ForgotPassword).
* `github.com/aws/aws-sdk-go-v2/service/drs`: [v1.6.0](service/drs/CHANGELOG.md#v160-2022-05-31)
  * **Feature**: Changed existing APIs and added new APIs to accommodate using multiple AWS accounts with AWS Elastic Disaster Recovery.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.22.0](service/iotsitewise/CHANGELOG.md#v1220-2022-05-31)
  * **Feature**: This release adds the following new optional field to the IoT SiteWise asset resource: assetDescription.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.15.0](service/lookoutmetrics/CHANGELOG.md#v1150-2022-05-31)
  * **Feature**: Adding backtest mode to detectors using the Cloudwatch data source.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.20.0](service/transcribe/CHANGELOG.md#v1200-2022-05-31)
  * **Feature**: Amazon Transcribe now supports automatic language identification for multi-lingual audio in batch mode.

# Release (2022-05-27)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.16.0](service/appflow/CHANGELOG.md#v1160-2022-05-27)
  * **Feature**: Adding the following features/changes: Parquet output that preserves typing from the source connector, Failed executions threshold before deactivation for scheduled flows, increasing max size of access and refresh token from 2048 to 4096
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.17.0](service/datasync/CHANGELOG.md#v1170-2022-05-27)
  * **Feature**: AWS DataSync now supports TLS encryption in transit, file system policies and access points for EFS locations.
* `github.com/aws/aws-sdk-go-v2/service/emrserverless`: [v1.1.0](service/emrserverless/CHANGELOG.md#v110-2022-05-27)
  * **Feature**: This release adds support for Amazon EMR Serverless, a serverless runtime environment that simplifies running analytics applications using the latest open source frameworks such as Apache Spark and Apache Hive.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.32.0](service/sagemaker/CHANGELOG.md#v1320-2022-05-27)
  * **Feature**: Amazon SageMaker Notebook Instances now allows configuration of Instance Metadata Service version and Amazon SageMaker Studio now supports G5 instance types.

# Release (2022-05-26)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.45.0](service/ec2/CHANGELOG.md#v1450-2022-05-26)
  * **Feature**: C7g instances, powered by the latest generation AWS Graviton3 processors, provide the best price performance in Amazon EC2 for compute-intensive workloads.
* `github.com/aws/aws-sdk-go-v2/service/emrserverless`: [v1.0.0](service/emrserverless/CHANGELOG.md#v100-2022-05-26)
  * **Release**: New AWS service client module
  * **Feature**: This release adds support for Amazon EMR Serverless, a serverless runtime environment that simplifies running analytics applications using the latest open source frameworks such as Apache Spark and Apache Hive.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.21.0](service/forecast/CHANGELOG.md#v1210-2022-05-26)
  * **Feature**: Introduced a new field in Auto Predictor as Time Alignment Boundary. It helps in aligning the timestamps generated during Forecast exports
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.22.0](service/lightsail/CHANGELOG.md#v1220-2022-05-26)
  * **Feature**: Amazon Lightsail now supports the ability to configure a Lightsail Container Service to pull images from Amazon ECR private repositories in your account.

# Release (2022-05-25)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.15.6](service/apigateway/CHANGELOG.md#v1156-2022-05-25)
  * **Documentation**: Documentation updates for Amazon API Gateway
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.12.3](service/apprunner/CHANGELOG.md#v1123-2022-05-25)
  * **Documentation**: Documentation-only update added for CodeConfiguration.
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.21.0](service/cloudformation/CHANGELOG.md#v1210-2022-05-25)
  * **Feature**: Add a new parameter statusReason to DescribeStackSetOperation output for additional details
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.24.0](service/fsx/CHANGELOG.md#v1240-2022-05-25)
  * **Feature**: This release adds root squash support to FSx for Lustre to restrict root level access from clients by mapping root users to a less-privileged user/group with limited permissions.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.14.0](service/lookoutmetrics/CHANGELOG.md#v1140-2022-05-25)
  * **Feature**: Adding AthenaSourceConfig for MetricSet APIs to support Athena as a data source.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.31.0](service/sagemaker/CHANGELOG.md#v1310-2022-05-25)
  * **Feature**: Amazon SageMaker Autopilot adds support for manually selecting features from the input dataset using the CreateAutoMLJob API.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.9](service/secretsmanager/CHANGELOG.md#v1159-2022-05-25)
  * **Documentation**: Documentation updates for Secrets Manager
* `github.com/aws/aws-sdk-go-v2/service/voiceid`: [v1.9.0](service/voiceid/CHANGELOG.md#v190-2022-05-25)
  * **Feature**: VoiceID will now automatically expire Speakers if they haven't been accessed for Enrollment, Re-enrollment or Successful Auth for three years. The Speaker APIs now return a "LastAccessedAt" time for Speakers, and the EvaluateSession API returns "SPEAKER_EXPIRED" Auth Decision for EXPIRED Speakers.

# Release (2022-05-24)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.16.0](service/cognitoidentityprovider/CHANGELOG.md#v1160-2022-05-24)
  * **Feature**: Amazon Cognito now supports requiring attribute verification (ex. email and phone number) before update.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.44.0](service/ec2/CHANGELOG.md#v1440-2022-05-24)
  * **Feature**: Stop Protection feature enables customers to protect their instances from accidental stop actions.
* `github.com/aws/aws-sdk-go-v2/service/ivschat`: [v1.0.4](service/ivschat/CHANGELOG.md#v104-2022-05-24)
  * **Documentation**: Doc-only update. For MessageReviewHandler structure, added timeout period in the description of the fallbackResult field
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.24.0](service/mediaconvert/CHANGELOG.md#v1240-2022-05-24)
  * **Feature**: AWS Elemental MediaConvert SDK has added support for rules that constrain Automatic-ABR rendition selection when generating ABR package ladders.
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.13.0](service/networkmanager/CHANGELOG.md#v1130-2022-05-24)
  * **Feature**: This release adds Multi Account API support for a TGW Global Network, to enable and disable AWSServiceAccess with AwsOrganizations for Network Manager service and dependency CloudFormation StackSets service.

# Release (2022-05-23)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.21.0](service/elasticache/CHANGELOG.md#v1210-2022-05-23)
  * **Feature**: Added support for encryption in transit for Memcached clusters. Customers can now launch Memcached cluster with encryption in transit enabled when using Memcached version 1.6.12 or later.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.20.0](service/forecast/CHANGELOG.md#v1200-2022-05-23)
  * **Feature**: New APIs for Monitor that help you understand how your predictors perform over time.
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.20.0](service/personalize/CHANGELOG.md#v1200-2022-05-23)
  * **Feature**: Adding modelMetrics as part of DescribeRecommender API response for Personalize.

# Release (2022-05-20)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.15.7](service/cloudwatchlogs/CHANGELOG.md#v1157-2022-05-20)
  * **Documentation**: Doc-only update to publish the new valid values for log retention
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.18.0](service/comprehend/CHANGELOG.md#v1180-2022-05-20)
  * **Feature**: Comprehend releases 14 new entity types for DetectPiiEntities and ContainsPiiEntities APIs.

# Release (2022-05-19)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/gamesparks`: [v1.1.0](service/gamesparks/CHANGELOG.md#v110-2022-05-19)
  * **Feature**: This release adds an optional DeploymentResult field in the responses of GetStageDeploymentIntegrationTests and ListStageDeploymentIntegrationTests APIs.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.13.0](service/lookoutmetrics/CHANGELOG.md#v1130-2022-05-19)
  * **Feature**: In this release we added SnsFormat to SNSConfiguration to support human readable alert.

# Release (2022-05-18)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.14.0](service/appmesh/CHANGELOG.md#v1140-2022-05-18)
  * **Feature**: This release updates the existing Create and Update APIs for meshes and virtual nodes by adding a new IP preference field. This new IP preference field can be used to control the IP versions being used with the mesh and allows for IPv6 support within App Mesh.
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.18.3](service/batch/CHANGELOG.md#v1183-2022-05-18)
  * **Documentation**: Documentation updates for AWS Batch.
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.16.0](service/greengrassv2/CHANGELOG.md#v1160-2022-05-18)
  * **Feature**: This release adds the new DeleteDeployment API operation that you can use to delete deployment resources. This release also adds support for discontinued AWS-provided components, so AWS can communicate when a component has any issues that you should consider before you deploy it.
* `github.com/aws/aws-sdk-go-v2/service/ioteventsdata`: [v1.12.0](service/ioteventsdata/CHANGELOG.md#v1120-2022-05-18)
  * **Feature**: Introducing new API for deleting detectors: BatchDeleteDetector.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.22.0](service/quicksight/CHANGELOG.md#v1220-2022-05-18)
  * **Feature**: API UpdatePublicSharingSettings enables IAM admins to enable/disable account level setting for public access of dashboards. When enabled, owners/co-owners for dashboards can enable public access on their dashboards. These dashboards can only be accessed through share link or embedding.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.19.0](service/transfer/CHANGELOG.md#v1190-2022-05-18)
  * **Feature**: AWS Transfer Family now supports SetStat server configuration option, which provides the ability to ignore SetStat command issued by file transfer clients, enabling customers to upload files without any errors.

# Release (2022-05-17)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/internal/ini`: [v1.3.12](internal/ini/CHANGELOG.md#v1312-2022-05-17)
  * **Bug Fix**: Removes the fuzz testing files from the module, as they are invalid and not used.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.25.0](service/glue/CHANGELOG.md#v1250-2022-05-17)
  * **Feature**: This release adds a new optional parameter called codeGenNodeConfiguration to CRUD job APIs that allows users to manage visual jobs via APIs. The updated CreateJob and UpdateJob will create jobs that can be viewed in Glue Studio as a visual graph. GetJob can be used to get codeGenNodeConfiguration.
* `github.com/aws/aws-sdk-go-v2/service/iotsecuretunneling`: [v1.13.1](service/iotsecuretunneling/CHANGELOG.md#v1131-2022-05-17)
  * **Bug Fix**: Fixes iotsecuretunneling and mobile API clients to use the correct name for signing requests, Fixes [#1686](https://github.com/aws/aws-sdk-go-v2/issues/1686).
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.17.2](service/kms/CHANGELOG.md#v1172-2022-05-17)
  * **Documentation**: Add HMAC best practice tip, annual rotation of AWS managed keys.
* `github.com/aws/aws-sdk-go-v2/service/mobile`: [v1.11.5](service/mobile/CHANGELOG.md#v1115-2022-05-17)
  * **Bug Fix**: Fixes iotsecuretunneling and mobile API clients to use the correct name for signing requests, Fixes [#1686](https://github.com/aws/aws-sdk-go-v2/issues/1686).

# Release (2022-05-16)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/applicationdiscoveryservice`: [v1.13.0](service/applicationdiscoveryservice/CHANGELOG.md#v1130-2022-05-16)
  * **Feature**: Add Migration Evaluator Collector details to the GetDiscoverySummary API response
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.18.0](service/cloudfront/CHANGELOG.md#v1180-2022-05-16)
  * **Feature**: Introduced a new error (TooLongCSPInResponseHeadersPolicy) that is returned when the value of the Content-Security-Policy header in a response headers policy exceeds the maximum allowed length.
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.18.1](service/rekognition/CHANGELOG.md#v1181-2022-05-16)
  * **Documentation**: Documentation updates for Amazon Rekognition.
* `github.com/aws/aws-sdk-go-v2/service/resiliencehub`: [v1.6.0](service/resiliencehub/CHANGELOG.md#v160-2022-05-16)
  * **Feature**: In this release, we are introducing support for Amazon Elastic Container Service, Amazon Route 53, AWS Elastic Disaster Recovery, AWS Backup in addition to the existing supported Services.  This release also supports Terraform file input from S3 and scheduling daily assessments
* `github.com/aws/aws-sdk-go-v2/service/servicecatalog`: [v1.14.2](service/servicecatalog/CHANGELOG.md#v1142-2022-05-16)
  * **Documentation**: Updated the descriptions for the ListAcceptedPortfolioShares API description and the PortfolioShareType parameters.
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.16.5](service/sts/CHANGELOG.md#v1165-2022-05-16)
  * **Documentation**: Documentation updates for AWS Security Token Service.
* `github.com/aws/aws-sdk-go-v2/service/workspacesweb`: [v1.6.0](service/workspacesweb/CHANGELOG.md#v160-2022-05-16)
  * **Feature**: Amazon WorkSpaces Web now supports Administrator timeout control

# Release (2022-05-13)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/grafana`: [v1.9.0](service/grafana/CHANGELOG.md#v190-2022-05-13)
  * **Feature**: This release adds APIs for creating and deleting API keys in an Amazon Managed Grafana workspace.

# Release (2022-05-12)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.43.0](service/ec2/CHANGELOG.md#v1430-2022-05-12)
  * **Feature**: This release introduces a target type Gateway Load Balancer Endpoint for mirrored traffic. Customers can now specify GatewayLoadBalancerEndpoint option during the creation of a traffic mirror target.
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.10.5](service/finspacedata/CHANGELOG.md#v1105-2022-05-12)
  * **Documentation**: We've now deprecated CreateSnapshot permission for creating a data view, instead use CreateDataView permission.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.25.1](service/iot/CHANGELOG.md#v1251-2022-05-12)
  * **Documentation**: Documentation update for China region ListMetricValues for IoT
* `github.com/aws/aws-sdk-go-v2/service/ivschat`: [v1.0.2](service/ivschat/CHANGELOG.md#v102-2022-05-12)
  * **Documentation**: Documentation-only updates for IVS Chat API Reference.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.27.0](service/kendra/CHANGELOG.md#v1270-2022-05-12)
  * **Feature**: Amazon Kendra now provides a data source connector for Jira. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-jira.html
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.23.0](service/lambda/CHANGELOG.md#v1230-2022-05-12)
  * **Feature**: Lambda releases NodeJs 16 managed runtime to be available in all commercial regions.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.21.0](service/lightsail/CHANGELOG.md#v1210-2022-05-12)
  * **Feature**: This release adds support to include inactive database bundles in the response of the GetRelationalDatabaseBundles request.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.19.1](service/outposts/CHANGELOG.md#v1191-2022-05-12)
  * **Documentation**: Documentation updates for AWS Outposts.
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.14.0](service/ssmincidents/CHANGELOG.md#v1140-2022-05-12)
  * **Feature**: Adding support for dynamic SSM Runbook parameter values. Updating validation pattern for engagements. Adding ConflictException to UpdateReplicationSet API contract.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.18.6](service/transfer/CHANGELOG.md#v1186-2022-05-12)
  * **Documentation**: AWS Transfer Family now accepts ECDSA keys for server host keys

# Release (2022-05-11)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.42.0](service/ec2/CHANGELOG.md#v1420-2022-05-11)
  * **Feature**: This release updates AWS PrivateLink APIs to support IPv6 for PrivateLink Services and Endpoints of type 'Interface'.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.7](service/secretsmanager/CHANGELOG.md#v1157-2022-05-11)
  * **Documentation**: Doc only update for Secrets Manager that fixes several customer-reported issues.

# Release (2022-05-10)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.17.5](service/computeoptimizer/CHANGELOG.md#v1175-2022-05-10)
  * **Documentation**: Documentation updates for Compute Optimizer
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.41.0](service/ec2/CHANGELOG.md#v1410-2022-05-10)
  * **Feature**: Added support for using NitroTPM and UEFI Secure Boot on EC2 instances.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.21.0](service/eks/CHANGELOG.md#v1210-2022-05-10)
  * **Feature**: Adds BOTTLEROCKET_ARM_64_NVIDIA and BOTTLEROCKET_x86_64_NVIDIA AMI types to EKS managed nodegroups
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.18.0](service/emr/CHANGELOG.md#v1180-2022-05-10)
  * **Feature**: This release updates the Amazon EMR ModifyInstanceGroups API to support "MERGE" type cluster reconfiguration. Also, added the ability to specify a particular Amazon Linux release for all nodes in a cluster launch request.
* `github.com/aws/aws-sdk-go-v2/service/migrationhubrefactorspaces`: [v1.5.5](service/migrationhubrefactorspaces/CHANGELOG.md#v155-2022-05-10)
  * **Documentation**: AWS Migration Hub Refactor Spaces documentation only update to fix a formatting issue.

# Release (2022-05-09)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.15.5](config/CHANGELOG.md#v1155-2022-05-09)
  * **Bug Fix**: Fixes a bug in LoadDefaultConfig to correctly assign ConfigSources so all config resolvers have access to the config sources. This fixes the feature/ec2/imds client not having configuration applied via config.LoadOptions such as EC2IMDSClientEnableState. PR [#1682](https://github.com/aws/aws-sdk-go-v2/pull/1682)
* `github.com/aws/aws-sdk-go-v2/service/cloudcontrol`: [v1.10.0](service/cloudcontrol/CHANGELOG.md#v1100-2022-05-09)
  * **Feature**: SDK release for Cloud Control API to include paginators for Python SDK.
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.7.0](service/evidently/CHANGELOG.md#v170-2022-05-09)
  * **Feature**: Add detail message inside GetExperimentResults API response to indicate experiment result availability
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.13.5](service/ssmcontacts/CHANGELOG.md#v1135-2022-05-09)
  * **Documentation**: Fixed an error in the DescribeEngagement example for AWS Incident Manager.

# Release (2022-05-06)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.40.0](service/ec2/CHANGELOG.md#v1400-2022-05-06)
  * **Feature**: Add new state values for IPAMs, IPAM Scopes, and IPAM Pools.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.17.0](service/location/CHANGELOG.md#v1170-2022-05-06)
  * **Feature**: Amazon Location Service now includes a MaxResults parameter for ListGeofences requests.
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.16.0](service/mediapackage/CHANGELOG.md#v1160-2022-05-06)
  * **Feature**: This release adds Dvb Dash 2014 as an available profile option for Dash Origin Endpoints.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.21.1](service/rds/CHANGELOG.md#v1211-2022-05-06)
  * **Documentation**: Various documentation improvements.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.24.0](service/redshift/CHANGELOG.md#v1240-2022-05-06)
  * **Feature**: Introduces new field 'LoadSampleData' in CreateCluster operation. Customers can now specify 'LoadSampleData' option during creation of a cluster, which results in loading of sample data in the cluster that is created.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.21.1](service/securityhub/CHANGELOG.md#v1211-2022-05-06)
  * **Documentation**: Documentation updates for Security Hub API reference

# Release (2022-05-05)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.16.0](service/datasync/CHANGELOG.md#v1160-2022-05-05)
  * **Feature**: AWS DataSync now supports a new ObjectTags Task API option that can be used to control whether Object Tags are transferred.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.39.0](service/ec2/CHANGELOG.md#v1390-2022-05-05)
  * **Feature**: Amazon EC2 I4i instances are powered by 3rd generation Intel Xeon Scalable processors and feature up to 30 TB of local AWS Nitro SSD storage
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.25.0](service/iot/CHANGELOG.md#v1250-2022-05-05)
  * **Feature**: AWS IoT Jobs now allows you to create up to 100,000 active continuous and snapshot jobs by using concurrency control.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.26.0](service/kendra/CHANGELOG.md#v1260-2022-05-05)
  * **Feature**: AWS Kendra now supports hierarchical facets for a query. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/filtering.html

# Release (2022-05-04)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.16.0](service/backup/CHANGELOG.md#v1160-2022-05-04)
  * **Feature**: Adds support to 2 new filters about job complete time for 3 list jobs APIs in AWS Backup
* `github.com/aws/aws-sdk-go-v2/service/iotsecuretunneling`: [v1.13.0](service/iotsecuretunneling/CHANGELOG.md#v1130-2022-05-04)
  * **Feature**: This release introduces a new API RotateTunnelAccessToken that allow revoking the existing tokens and generate new tokens
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.20.1](service/lightsail/CHANGELOG.md#v1201-2022-05-04)
  * **Documentation**: Documentation updates for Lightsail
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.27.0](service/ssm/CHANGELOG.md#v1270-2022-05-04)
  * **Feature**: This release adds the TargetMaps parameter in SSM State Manager API.

# Release (2022-05-03)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.38.0](service/ec2/CHANGELOG.md#v1380-2022-05-03)
  * **Feature**: Adds support for allocating Dedicated Hosts on AWS  Outposts. The AllocateHosts API now accepts an OutpostArn request  parameter, and the DescribeHosts API now includes an OutpostArn response parameter.
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideo`: [v1.12.0](service/kinesisvideo/CHANGELOG.md#v1120-2022-05-03)
  * **Feature**: Add support for multiple image feature related APIs for configuring image generation and notification of a video stream. Add "GET_IMAGES" to the list of supported API names for the GetDataEndpoint API.
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideoarchivedmedia`: [v1.13.0](service/kinesisvideoarchivedmedia/CHANGELOG.md#v1130-2022-05-03)
  * **Feature**: Add support for GetImages API  for retrieving images from a video stream
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.26.8](service/s3/CHANGELOG.md#v1268-2022-05-03)
  * **Documentation**: Documentation only update for doc bug fixes for the S3 API docs.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.30.0](service/sagemaker/CHANGELOG.md#v1300-2022-05-03)
  * **Feature**: SageMaker Autopilot adds new metrics for all candidate models generated by Autopilot experiments; RStudio on SageMaker now allows users to bring your own development environment in a custom image.

# Release (2022-05-02)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/organizations`: [v1.16.0](service/organizations/CHANGELOG.md#v1160-2022-05-02)
  * **Feature**: This release adds the INVALID_PAYMENT_INSTRUMENT as a fail reason and an error message.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.19.0](service/outposts/CHANGELOG.md#v1190-2022-05-02)
  * **Feature**: This release adds a new API called ListAssets to the Outposts SDK, which lists the hardware assets in an Outpost.
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.15.0](service/synthetics/CHANGELOG.md#v1150-2022-05-02)
  * **Feature**: CloudWatch Synthetics has introduced a new feature to provide customers with an option to delete the underlying resources that Synthetics canary creates when the user chooses to delete the canary.

# Release (2022-04-29)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/codegurureviewer`: [v1.16.0](service/codegurureviewer/CHANGELOG.md#v1160-2022-04-29)
  * **Feature**: Amazon CodeGuru Reviewer now supports suppressing recommendations from being generated on specific files and directories.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.23.0](service/mediaconvert/CHANGELOG.md#v1230-2022-04-29)
  * **Feature**: AWS Elemental MediaConvert SDK nows supports creation of Dolby Vision profile 8.1, the ability to generate black frames of video, and introduces audio-only DASH and CMAF support.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.21.0](service/rds/CHANGELOG.md#v1210-2022-04-29)
  * **Feature**: Feature - Adds support for Internet Protocol Version 6 (IPv6) on RDS database instances.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.26.0](service/ssm/CHANGELOG.md#v1260-2022-04-29)
  * **Feature**: Update the StartChangeRequestExecution, adding TargetMaps to the Runbook parameter
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.20.0](service/wafv2/CHANGELOG.md#v1200-2022-04-29)
  * **Feature**: You can now inspect all request headers and all cookies. You can now specify how to handle oversize body contents in your rules that inspect the body.

# Release (2022-04-28)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.18.5](service/auditmanager/CHANGELOG.md#v1185-2022-04-28)
  * **Documentation**: This release adds documentation updates for Audit Manager. We provided examples of how to use the Custom_ prefix for the keywordValue attribute. We also provided more details about the DeleteAssessmentReport operation.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.16.0](service/braket/CHANGELOG.md#v1160-2022-04-28)
  * **Feature**: This release enables Braket Hybrid Jobs with Embedded Simulators to have multiple instances.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.24.0](service/connect/CHANGELOG.md#v1240-2022-04-28)
  * **Feature**: This release introduces an API for changing the current agent status of a user in Connect.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.37.0](service/ec2/CHANGELOG.md#v1370-2022-04-28)
  * **Feature**: This release adds support to query the public key and creation date of EC2 Key Pairs. Additionally, the format (pem or ppk) of a key pair can be specified when creating a new key pair.
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.13.5](service/guardduty/CHANGELOG.md#v1135-2022-04-28)
  * **Documentation**: Documentation update for API description.
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.17.0](service/networkfirewall/CHANGELOG.md#v1170-2022-04-28)
  * **Feature**: AWS Network Firewall adds support for stateful threat signature AWS managed rule groups.

# Release (2022-04-27)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.11.5](service/amplify/CHANGELOG.md#v1115-2022-04-27)
  * **Documentation**: Documentation only update to support the Amplify GitHub App feature launch
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmediapipelines`: [v1.0.0](service/chimesdkmediapipelines/CHANGELOG.md#v100-2022-04-27)
  * **Release**: New AWS service client module
  * **Feature**: For Amazon Chime SDK meetings, the Amazon Chime Media Pipelines SDK allows builders to capture audio, video, and content share streams. You can also capture meeting events, live transcripts, and data messages. The pipelines save the artifacts to an Amazon S3 bucket that you designate.
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.16.0](service/cloudtrail/CHANGELOG.md#v1160-2022-04-27)
  * **Feature**: Increases the retention period maximum to 2557 days. Deprecates unused fields of the ListEventDataStores API response. Updates documentation.
* `github.com/aws/aws-sdk-go-v2/service/internal/checksum`: [v1.1.5](service/internal/checksum/CHANGELOG.md#v115-2022-04-27)
  * **Bug Fix**: Fixes a bug that could cause the SigV4 payload hash to be incorrectly encoded, leading to signing errors.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.19.0](service/iotwireless/CHANGELOG.md#v1190-2022-04-27)
  * **Feature**: Add list support for event configurations, allow to get and update event configurations by resource type, support LoRaWAN events; Make NetworkAnalyzerConfiguration as a resource, add List, Create, Delete API support; Add FCntStart attribute support for ABP WirelessDevice.
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.13.0](service/lookoutequipment/CHANGELOG.md#v1130-2022-04-27)
  * **Feature**: This release adds the following new features: 1) Introduces an option for automatic schema creation 2) Now allows for Ingestion of data containing most common errors and allows automatic data cleaning 3) Introduces new API ListSensorStatistics that gives further information about the ingested data
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.18.0](service/rekognition/CHANGELOG.md#v1180-2022-04-27)
  * **Feature**: This release adds support to configure stream-processor resources for label detections on streaming-videos. UpateStreamProcessor API is also launched with this release, which could be used to update an existing stream-processor.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.29.0](service/sagemaker/CHANGELOG.md#v1290-2022-04-27)
  * **Feature**: Amazon SageMaker Autopilot adds support for custom validation dataset and validation ratio through the CreateAutoMLJob and DescribeAutoMLJob APIs.

# Release (2022-04-26)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.17.0](service/cloudfront/CHANGELOG.md#v1170-2022-04-26)
  * **Feature**: CloudFront now supports the Server-Timing header in HTTP responses sent from CloudFront. You can use this header to view metrics that help you gain insights about the behavior and performance of CloudFront. To use this header, enable it in a response headers policy.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.24.2](service/glue/CHANGELOG.md#v1242-2022-04-26)
  * **Documentation**: This release adds documentation for the APIs to create, read, delete, list, and batch read of AWS Glue custom patterns, and for Lake Formation configuration settings in the AWS Glue crawler.
* `github.com/aws/aws-sdk-go-v2/service/ivschat`: [v1.0.0](service/ivschat/CHANGELOG.md#v100-2022-04-26)
  * **Release**: New AWS service client module
  * **Feature**: Adds new APIs for IVS Chat, a feature for building interactive chat experiences alongside an IVS broadcast.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.20.0](service/lightsail/CHANGELOG.md#v1200-2022-04-26)
  * **Feature**: This release adds support for Lightsail load balancer HTTP to HTTPS redirect and TLS policy configuration.
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.16.0](service/networkfirewall/CHANGELOG.md#v1160-2022-04-26)
  * **Feature**: AWS Network Firewall now enables customers to use a customer managed AWS KMS key for the encryption of their firewall resources.
* `github.com/aws/aws-sdk-go-v2/service/pricing`: [v1.14.5](service/pricing/CHANGELOG.md#v1145-2022-04-26)
  * **Documentation**: Documentation updates for Price List API
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.28.0](service/sagemaker/CHANGELOG.md#v1280-2022-04-26)
  * **Feature**: SageMaker Inference Recommender now accepts customer KMS key ID for encryption of endpoints and compilation outputs created during inference recommendation.

# Release (2022-04-25)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.16.3
  * **Dependency Update**: Update SDK's internal copy of golang.org/x/sync/singleflight to address issue with test failing due to timeing issues
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.12.0](credentials/CHANGELOG.md#v1120-2022-04-25)
  * **Feature**: Adds Duration and Policy options that can be used when creating stscreds.WebIdentityRoleProvider credentials provider.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.23.0](service/connect/CHANGELOG.md#v1230-2022-04-25)
  * **Feature**: This release adds SearchUsers API which can be used to search for users with a Connect Instance
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.14.4](service/gamelift/CHANGELOG.md#v1144-2022-04-25)
  * **Documentation**: Documentation updates for Amazon GameLift.
* `github.com/aws/aws-sdk-go-v2/service/mq`: [v1.13.0](service/mq/CHANGELOG.md#v1130-2022-04-25)
  * **Feature**: This release adds the CRITICAL_ACTION_REQUIRED broker state and the ActionRequired API property. CRITICAL_ACTION_REQUIRED informs you when your broker is degraded. ActionRequired provides you with a code which you can use to find instructions in the Developer Guide on how to resolve the issue.
* `github.com/aws/aws-sdk-go-v2/service/rdsdata`: [v1.12.0](service/rdsdata/CHANGELOG.md#v1120-2022-04-25)
  * **Feature**: Support to receive SQL query results in the form of a simplified JSON string. This enables developers using the new JSON string format to more easily convert it to an object using popular JSON string parsing libraries.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.21.0](service/securityhub/CHANGELOG.md#v1210-2022-04-25)
  * **Feature**: Security Hub now lets you opt-out of auto-enabling the defaults standards (CIS and FSBP) in accounts that are auto-enabled with Security Hub via Security Hub's integration with AWS Organizations.

# Release (2022-04-22)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.9.0](service/chimesdkmeetings/CHANGELOG.md#v190-2022-04-22)
  * **Feature**: Include additional exceptions types.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.36.0](service/ec2/CHANGELOG.md#v1360-2022-04-22)
  * **Feature**: Adds support for waiters that automatically poll for a deleted NAT Gateway until it reaches the deleted state.

# Release (2022-04-21)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.20.5](service/elasticache/CHANGELOG.md#v1205-2022-04-21)
  * **Documentation**: Doc only update for ElastiCache
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.24.0](service/glue/CHANGELOG.md#v1240-2022-04-21)
  * **Feature**: This release adds APIs to create, read, delete, list, and batch read of Glue custom entity types
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.21.0](service/iotsitewise/CHANGELOG.md#v1210-2022-04-21)
  * **Feature**: This release adds 3 new batch data query APIs : BatchGetAssetPropertyValue, BatchGetAssetPropertyValueHistory and BatchGetAssetPropertyAggregates
* `github.com/aws/aws-sdk-go-v2/service/iottwinmaker`: [v1.7.0](service/iottwinmaker/CHANGELOG.md#v170-2022-04-21)
  * **Feature**: General availability (GA) for AWS IoT TwinMaker. For more information, see https://docs.aws.amazon.com/iot-twinmaker/latest/apireference/Welcome.html
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.12.0](service/lookoutmetrics/CHANGELOG.md#v1120-2022-04-21)
  * **Feature**: Added DetectMetricSetConfig API for detecting configuration required for creating metric set from provided S3 data source.
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.17.0](service/mediatailor/CHANGELOG.md#v1170-2022-04-21)
  * **Feature**: This release introduces tiered channels and adds support for live sources. Customers using a STANDARD channel can now create programs using live sources.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.5](service/secretsmanager/CHANGELOG.md#v1155-2022-04-21)
  * **Documentation**: Documentation updates for Secrets Manager
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.17.0](service/storagegateway/CHANGELOG.md#v1170-2022-04-21)
  * **Feature**: This release adds support for minimum of 5 character length virtual tape barcodes.
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.8.0](service/wisdom/CHANGELOG.md#v180-2022-04-21)
  * **Feature**: This release updates the GetRecommendations API to include a trigger event list for classifying and grouping recommendations.

# Release (2022-04-20)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.22.0](service/connect/CHANGELOG.md#v1220-2022-04-20)
  * **Feature**: This release adds APIs to search, claim, release, list, update, and describe phone numbers. You can also use them to associate and disassociate contact flows to phone numbers.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.21.0](service/macie2/CHANGELOG.md#v1210-2022-04-20)
  * **Feature**: Sensitive data findings in Amazon Macie now indicate how Macie found the sensitive data that produced a finding (originType).
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.14.0](service/mgn/CHANGELOG.md#v1140-2022-04-20)
  * **Feature**: Removed required annotation from input fields in Describe operations requests. Added quotaValue to ServiceQuotaExceededException
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.20.0](service/rds/CHANGELOG.md#v1200-2022-04-20)
  * **Feature**: Added a new cluster-level attribute to set the capacity range for Aurora Serverless v2 instances.

# Release (2022-04-19)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.23.0](service/autoscaling/CHANGELOG.md#v1230-2022-04-19)
  * **Feature**: EC2 Auto Scaling now adds default instance warm-up times for all scaling activities, health check replacements, and other replacement events in the Auto Scaling instance lifecycle.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.25.0](service/kendra/CHANGELOG.md#v1250-2022-04-19)
  * **Feature**: Amazon Kendra now provides a data source connector for Quip. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-quip.html
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.17.0](service/kms/CHANGELOG.md#v1170-2022-04-19)
  * **Feature**: Adds support for KMS keys and APIs that generate and verify HMAC codes
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.19.0](service/personalize/CHANGELOG.md#v1190-2022-04-19)
  * **Feature**: Adding StartRecommender and StopRecommender APIs for Personalize.
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.15.0](service/polly/CHANGELOG.md#v1150-2022-04-19)
  * **Feature**: Amazon Polly adds new Austrian German voice - Hannah. Hannah is available as Neural voice only.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.23.0](service/redshift/CHANGELOG.md#v1230-2022-04-19)
  * **Feature**: Introduces new fields for LogDestinationType and LogExports on EnableLogging requests and Enable/Disable/DescribeLogging responses. Customers can now select CloudWatch Logs as a destination for their Audit Logs.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.25.0](service/ssm/CHANGELOG.md#v1250-2022-04-19)
  * **Feature**: Added offset support for specifying the number of days to wait after the date and time specified by a CRON expression when creating SSM association.
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.15.0](service/textract/CHANGELOG.md#v1150-2022-04-19)
  * **Feature**: This release adds support for specifying and extracting information from documents using the Queries feature within Analyze Document API
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.18.4](service/transfer/CHANGELOG.md#v1184-2022-04-19)
  * **Documentation**: This release contains corrected HomeDirectoryMappings examples for several API functions: CreateAccess, UpdateAccess, CreateUser, and UpdateUser,.
* `github.com/aws/aws-sdk-go-v2/service/worklink`: [v1.12.0](service/worklink/CHANGELOG.md#v1120-2022-04-19)
  * **Feature**: Amazon WorkLink is no longer supported. This will be removed in a future version of the SDK.

# Release (2022-04-15)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.9.0](feature/dynamodb/attributevalue/CHANGELOG.md#v190-2022-04-15)
  * **Feature**: Support has been added for specifying a custom time format when encoding and decoding DynamoDB AttributeValues. Use `EncoderOptions.EncodeTime` to specify a custom time encoding function, and use `DecoderOptions.DecodeTime` for specifying how to handle the corresponding AttributeValues using the format. Thank you [Pablo Lopez](https://github.com/plopezlpz) for this contribution.
* `github.com/aws/aws-sdk-go-v2/feature/dynamodbstreams/attributevalue`: [v1.9.0](feature/dynamodbstreams/attributevalue/CHANGELOG.md#v190-2022-04-15)
  * **Feature**: Support has been added for specifying a custom time format when encoding and decoding DynamoDB AttributeValues. Use `EncoderOptions.EncodeTime` to specify a custom time encoding function, and use `DecoderOptions.DecodeTime` for specifying how to handle the corresponding AttributeValues using the format. Thank you [Pablo Lopez](https://github.com/plopezlpz) for this contribution.
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.15.0](service/athena/CHANGELOG.md#v1150-2022-04-15)
  * **Feature**: This release adds subfields, ErrorMessage, Retryable, to the AthenaError response object in the GetQueryExecution API when a query fails.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.19.0](service/lightsail/CHANGELOG.md#v1190-2022-04-15)
  * **Feature**: This release adds support to describe the synchronization status of the account-level block public access feature for your Amazon Lightsail buckets.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.19.0](service/rds/CHANGELOG.md#v1190-2022-04-15)
  * **Feature**: Removes Amazon RDS on VMware with the deletion of APIs related to Custom Availability Zones and Media installation

# Release (2022-04-14)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.15.0](service/appflow/CHANGELOG.md#v1150-2022-04-14)
  * **Feature**: Enables users to pass custom token URL parameters for Oauth2 authentication during create connector profile
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.16.0](service/appstream/CHANGELOG.md#v1160-2022-04-14)
  * **Feature**: Includes updates for create and update fleet APIs to manage the session scripts locations for Elastic fleets.
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.18.0](service/batch/CHANGELOG.md#v1180-2022-04-14)
  * **Feature**: Enables configuration updates for compute environments with BEST_FIT_PROGRESSIVE and SPOT_CAPACITY_OPTIMIZED allocation strategies.
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.18.1](service/cloudwatch/CHANGELOG.md#v1181-2022-04-14)
  * **Documentation**: Updates documentation for additional statistics in CloudWatch Metric Streams.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.35.1](service/ec2/CHANGELOG.md#v1351-2022-04-14)
  * **Documentation**: Documentation updates for Amazon EC2.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.23.0](service/glue/CHANGELOG.md#v1230-2022-04-14)
  * **Feature**: Auto Scaling for Glue version 3.0 and later jobs to dynamically scale compute resources. This SDK change provides customers with the auto-scaled DPU usage

# Release (2022-04-13)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.18.0](service/cloudwatch/CHANGELOG.md#v1180-2022-04-13)
  * **Feature**: Adds support for additional statistics in CloudWatch Metric Streams.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.23.0](service/fsx/CHANGELOG.md#v1230-2022-04-13)
  * **Feature**: This release adds support for deploying FSx for ONTAP file systems in a single Availability Zone.

# Release (2022-04-12)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.17.0](service/devopsguru/CHANGELOG.md#v1170-2022-04-12)
  * **Feature**: This release adds new APIs DeleteInsight to deletes the insight along with the associated anomalies, events and recommendations.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.35.0](service/ec2/CHANGELOG.md#v1350-2022-04-12)
  * **Feature**: X2idn and X2iedn instances are powered by 3rd generation Intel Xeon Scalable processors with an all-core turbo frequency up to 3.5 GHzAmazon EC2. C6a instances are powered by 3rd generation AMD EPYC processors.
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.17.0](service/efs/CHANGELOG.md#v1170-2022-04-12)
  * **Feature**: Amazon EFS adds support for a ThrottlingException when using the CreateAccessPoint API if the account is nearing the AccessPoint limit(120).
* `github.com/aws/aws-sdk-go-v2/service/iottwinmaker`: [v1.6.0](service/iottwinmaker/CHANGELOG.md#v160-2022-04-12)
  * **Feature**: This release adds the following new features: 1) ListEntities API now supports search using ExternalId. 2) BatchPutPropertyValue and GetPropertyValueHistory API now allows users to represent time in sub-second level precisions.
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.15.4](service/kinesis/CHANGELOG.md#v1154-2022-04-12)
  * **Bug Fix**: Fixes an issue that caused the unexported constructor function names for EventStream types to be swapped for the event reader and writer respectivly.
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.14.4](service/lexruntimev2/CHANGELOG.md#v1144-2022-04-12)
  * **Bug Fix**: Fixes an issue that caused the unexported constructor function names for EventStream types to be swapped for the event reader and writer respectivly.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.26.5](service/s3/CHANGELOG.md#v1265-2022-04-12)
  * **Bug Fix**: Fixes an issue that caused the unexported constructor function names for EventStream types to be swapped for the event reader and writer respectivly.
* `github.com/aws/aws-sdk-go-v2/service/transcribestreaming`: [v1.6.4](service/transcribestreaming/CHANGELOG.md#v164-2022-04-12)
  * **Bug Fix**: Fixes an issue that caused the unexported constructor function names for EventStream types to be swapped for the event reader and writer respectivly.

# Release (2022-04-11)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/amplifyuibuilder`: [v1.6.0](service/amplifyuibuilder/CHANGELOG.md#v160-2022-04-11)
  * **Feature**: In this release, we have added the ability to bind events to component level actions.
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.12.0](service/apprunner/CHANGELOG.md#v1120-2022-04-11)
  * **Feature**: This release adds tracing for App Runner services with X-Ray using AWS Distro for OpenTelemetry. New APIs: CreateObservabilityConfiguration, DescribeObservabilityConfiguration, ListObservabilityConfigurations, and DeleteObservabilityConfiguration. Updated APIs: CreateService and UpdateService.
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.18.0](service/workspaces/CHANGELOG.md#v1180-2022-04-11)
  * **Feature**: Added API support that allows customers to create GPU-enabled WorkSpaces using EC2 G4dn instances.

# Release (2022-04-08)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.22.0](service/mediaconvert/CHANGELOG.md#v1220-2022-04-08)
  * **Feature**: AWS Elemental MediaConvert SDK has added support for the pass-through of WebVTT styling to WebVTT outputs, pass-through of KLV metadata to supported formats, and improved filter support for processing 444/RGB content.
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.17.0](service/mediapackagevod/CHANGELOG.md#v1170-2022-04-08)
  * **Feature**: This release adds ScteMarkersSource as an available field for Dash Packaging Configurations. When set to MANIFEST, MediaPackage will source the SCTE-35 markers from the manifest. When set to SEGMENTS, MediaPackage will source the SCTE-35 markers from the segments.
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.19.0](service/wafv2/CHANGELOG.md#v1190-2022-04-08)
  * **Feature**: Add a new CurrentDefaultVersion field to ListAvailableManagedRuleGroupVersions API response; add a new VersioningSupported boolean to each ManagedRuleGroup returned from ListAvailableManagedRuleGroups API response.

# Release (2022-04-07)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/internal/v4a`: [v1.0.0](internal/v4a/CHANGELOG.md#v100-2022-04-07)
  * **Release**: New internal v4a signing module location.
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.18.0](service/docdb/CHANGELOG.md#v1180-2022-04-07)
  * **Feature**: Added support to enable/disable performance insights when creating or modifying db instances
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.16.0](service/eventbridge/CHANGELOG.md#v1160-2022-04-07)
  * **Feature**: Adds new EventBridge Endpoint resources for disaster recovery, multi-region failover, and cross-region replication capabilities to help you build resilient event-driven applications.
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.18.0](service/personalize/CHANGELOG.md#v1180-2022-04-07)
  * **Feature**: This release provides tagging support in AWS Personalize.
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.14.4](service/pi/CHANGELOG.md#v1144-2022-04-07)
  * **Documentation**: Adds support for DocumentDB to the Performance Insights API.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.27.0](service/sagemaker/CHANGELOG.md#v1270-2022-04-07)
  * **Feature**: Amazon Sagemaker Notebook Instances now supports G5 instance types

# Release (2022-04-06)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.21.0](service/configservice/CHANGELOG.md#v1210-2022-04-06)
  * **Feature**: Add resourceType enums for AWS::EMR::SecurityConfiguration and AWS::SageMaker::CodeRepository
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.24.0](service/kendra/CHANGELOG.md#v1240-2022-04-06)
  * **Feature**: Amazon Kendra now provides a data source connector for Box. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-box.html
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.22.0](service/lambda/CHANGELOG.md#v1220-2022-04-06)
  * **Feature**: This release adds new APIs for creating and managing Lambda Function URLs and adds a new FunctionUrlAuthType parameter to the AddPermission API. Customers can use Function URLs to create built-in HTTPS endpoints on their functions.
* `github.com/aws/aws-sdk-go-v2/service/panorama`: [v1.7.0](service/panorama/CHANGELOG.md#v170-2022-04-06)
  * **Feature**: Added Brand field to device listings.

# Release (2022-04-05)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.15.0](service/datasync/CHANGELOG.md#v1150-2022-04-05)
  * **Feature**: AWS DataSync now supports Amazon FSx for OpenZFS locations.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.22.0](service/fsx/CHANGELOG.md#v1220-2022-04-05)
  * **Feature**: Provide customers more visibility into file system status by adding new "Misconfigured Unavailable" status for Amazon FSx for Windows File Server.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.21.4](service/s3control/CHANGELOG.md#v1214-2022-04-05)
  * **Documentation**: Documentation-only update for doc bug fixes for the S3 Control API docs.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.20.0](service/securityhub/CHANGELOG.md#v1200-2022-04-05)
  * **Feature**: Added additional ASFF details for RdsSecurityGroup AutoScalingGroup, ElbLoadBalancer, CodeBuildProject and RedshiftCluster.

# Release (2022-04-04)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.24.0](service/iot/CHANGELOG.md#v1240-2022-04-04)
  * **Feature**: AWS IoT - AWS IoT Device Defender adds support to list metric datapoints collected for IoT devices through the ListMetricValues API
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.13.0](service/proton/CHANGELOG.md#v1130-2022-04-04)
  * **Feature**: SDK release to support tagging for AWS Proton Repository resource
* `github.com/aws/aws-sdk-go-v2/service/servicecatalog`: [v1.14.0](service/servicecatalog/CHANGELOG.md#v1140-2022-04-04)
  * **Feature**: This release adds ProvisioningArtifictOutputKeys to DescribeProvisioningParameters to reference the outputs of a Provisioned Product and deprecates ProvisioningArtifactOutputs.
* `github.com/aws/aws-sdk-go-v2/service/sms`: [v1.12.4](service/sms/CHANGELOG.md#v1124-2022-04-04)
  * **Documentation**: Revised product update notice for SMS console deprecation.

# Release (2022-04-01)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.21.0](service/connect/CHANGELOG.md#v1210-2022-04-01)
  * **Feature**: This release updates these APIs: UpdateInstanceAttribute, DescribeInstanceAttribute and ListInstanceAttributes. You can use it to programmatically enable/disable multi-party conferencing using attribute type MULTI_PARTY_CONFERENCING on the specified Amazon Connect instance.

# Release (2022-03-31)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.8.4](feature/dynamodb/attributevalue/CHANGELOG.md#v184-2022-03-31)
  * **Documentation**: Fixes documentation typos in Number type's helper methods
* `github.com/aws/aws-sdk-go-v2/feature/dynamodbstreams/attributevalue`: [v1.8.4](feature/dynamodbstreams/attributevalue/CHANGELOG.md#v184-2022-03-31)
  * **Documentation**: Fixes documentation typos in Number type's helper methods
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.18.3](service/auditmanager/CHANGELOG.md#v1183-2022-03-31)
  * **Documentation**: This release adds documentation updates for Audit Manager. The updates provide data deletion guidance when a customer deregisters Audit Manager or deregisters a delegated administrator.
* `github.com/aws/aws-sdk-go-v2/service/cloudcontrol`: [v1.9.0](service/cloudcontrol/CHANGELOG.md#v190-2022-03-31)
  * **Feature**: SDK release for Cloud Control API in Amazon Web Services China (Beijing) Region, operated by Sinnet, and Amazon Web Services China (Ningxia) Region, operated by NWCD
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.20.0](service/databrew/CHANGELOG.md#v1200-2022-03-31)
  * **Feature**: This AWS Glue Databrew release adds feature to support ORC as an input format.
* `github.com/aws/aws-sdk-go-v2/service/grafana`: [v1.8.0](service/grafana/CHANGELOG.md#v180-2022-03-31)
  * **Feature**: This release adds tagging support to the Managed Grafana service. New APIs: TagResource, UntagResource and ListTagsForResource. Updates: add optional field tags to support tagging while calling CreateWorkspace.
* `github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoicev2`: [v1.0.0](service/pinpointsmsvoicev2/CHANGELOG.md#v100-2022-03-31)
  * **Release**: New AWS service client module
  * **Feature**: Amazon Pinpoint now offers a version 2.0 suite of SMS and voice APIs, providing increased control over sending and configuration. This release is a new SDK for sending SMS and voice messages called PinpointSMSVoiceV2.
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycluster`: [v1.9.0](service/route53recoverycluster/CHANGELOG.md#v190-2022-03-31)
  * **Feature**: This release adds a new API "ListRoutingControls" to list routing control states using the highly reliable Route 53 ARC data plane endpoints.
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.17.0](service/workspaces/CHANGELOG.md#v1170-2022-03-31)
  * **Feature**: Added APIs that allow you to customize the logo, login message, and help links in the WorkSpaces client login page. To learn more, visit https://docs.aws.amazon.com/workspaces/latest/adminguide/customize-branding.html

# Release (2022-03-30)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.34.0](service/ec2/CHANGELOG.md#v1340-2022-03-30)
  * **Feature**: This release simplifies the auto-recovery configuration process enabling customers to set the recovery behavior to disabled or default
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.17.0](service/fms/CHANGELOG.md#v1170-2022-03-30)
  * **Feature**: AWS Firewall Manager now supports the configuration of third-party policies that can use either the centralized or distributed deployment models.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.21.0](service/fsx/CHANGELOG.md#v1210-2022-03-30)
  * **Feature**: This release adds support for modifying throughput capacity for FSx for ONTAP file systems.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.23.3](service/iot/CHANGELOG.md#v1233-2022-03-30)
  * **Documentation**: Doc only update for IoT that fixes customer-reported issues.
* `github.com/aws/aws-sdk-go-v2/service/iotdataplane`: [v1.12.0](service/iotdataplane/CHANGELOG.md#v1120-2022-03-30)
  * **Feature**: Update the default AWS IoT Core Data Plane endpoint from VeriSign signed to ATS signed. If you have firewalls with strict egress rules, configure the rules to grant you access to data-ats.iot.[region].amazonaws.com or data-ats.iot.[region].amazonaws.com.cn.

# Release (2022-03-29)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/organizations`: [v1.15.0](service/organizations/CHANGELOG.md#v1150-2022-03-29)
  * **Feature**: This release provides the new CloseAccount API that enables principals in the management account to close any member account within an organization.

# Release (2022-03-28)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.17.3](service/acmpca/CHANGELOG.md#v1173-2022-03-28)
  * **Documentation**: Updating service name entities
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.20.0](service/medialive/CHANGELOG.md#v1200-2022-03-28)
  * **Feature**: This release adds support for selecting a maintenance window.

# Release (2022-03-25)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.17.0](service/batch/CHANGELOG.md#v1170-2022-03-25)
  * **Feature**: Bug Fix: Fixed a bug where shapes were marked as unboxed and were not serialized and sent over the wire, causing an API error from the service.
  * This is a breaking change, and has been accepted due to the API operation not being usable due to the members modeled as unboxed (aka value) types. The update changes the members to boxed (aka pointer) types so that the zero value of the members can be handled correctly by the SDK and service. Your application will fail to compile with the updated module. To workaround this you'll need to update your application to use pointer types for the members impacted.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.33.0](service/ec2/CHANGELOG.md#v1330-2022-03-25)
  * **Feature**: This is release adds support for Amazon VPC Reachability Analyzer to analyze path through a Transit Gateway.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.24.0](service/ssm/CHANGELOG.md#v1240-2022-03-25)
  * **Feature**: This Patch Manager release supports creating, updating, and deleting Patch Baselines for Rocky Linux OS.

# Release (2022-03-24)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.20.0](service/configservice/CHANGELOG.md#v1200-2022-03-24)
  * **Feature**: Added new APIs GetCustomRulePolicy and GetOrganizationCustomRulePolicy, and updated existing APIs PutConfigRule, DescribeConfigRule, DescribeConfigRuleEvaluationStatus, PutOrganizationConfigRule, DescribeConfigRule to support a new feature for building AWS Config rules with AWS CloudFormation Guard
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.21.0](service/lambda/CHANGELOG.md#v1210-2022-03-24)
  * **Feature**: Adds support for increased ephemeral storage (/tmp) up to 10GB for Lambda functions. Customers can now provision up to 10 GB of ephemeral storage per function instance, a 20x increase over the previous limit of 512 MB.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.19.0](service/transcribe/CHANGELOG.md#v1190-2022-03-24)
  * **Feature**: This release adds an additional parameter for subtitling with Amazon Transcribe batch jobs: outputStartIndex.

# Release (2022-03-23)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.16.0
  * **Feature**: Update CredentialsCache to make use of two new optional CredentialsProvider interfaces to give the cache, per provider, behavior how the cache handles credentials that fail to refresh, and adjusting expires time. See [aws.CredentialsCache](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#CredentialsCache) for more details.
  * **Feature**: Update `ec2rolecreds` package's `Provider` to implememnt support for CredentialsCache new optional caching strategy interfaces, HandleFailRefreshCredentialsCacheStrategy and AdjustExpiresByCredentialsCacheStrategy.
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.11.0](credentials/CHANGELOG.md#v1110-2022-03-23)
  * **Feature**: Update `ec2rolecreds` package's `Provider` to implememnt support for CredentialsCache new optional caching strategy interfaces, HandleFailRefreshCredentialsCacheStrategy and AdjustExpiresByCredentialsCacheStrategy.
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.18.0](service/auditmanager/CHANGELOG.md#v1180-2022-03-23)
  * **Feature**: This release updates 1 API parameter, the SnsArn attribute. The character length and regex pattern for the SnsArn attribute have been updated, which enables you to deselect an SNS topic when using the UpdateSettings operation.
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.15.0](service/ebs/CHANGELOG.md#v1150-2022-03-23)
  * **Feature**: Increased the maximum supported value for the Timeout parameter of the StartSnapshot API from 60 minutes to 4320 minutes.  Changed the HTTP error code for ConflictException from 503 to 409.
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.20.2](service/elasticache/CHANGELOG.md#v1202-2022-03-23)
  * **Documentation**: Doc only update for ElastiCache
* `github.com/aws/aws-sdk-go-v2/service/gamesparks`: [v1.0.0](service/gamesparks/CHANGELOG.md#v100-2022-03-23)
  * **Release**: New AWS service client module
  * **Feature**: Released the preview of Amazon GameSparks, a fully managed AWS service that provides a multi-service backend for game developers.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.22.0](service/redshift/CHANGELOG.md#v1220-2022-03-23)
  * **Feature**: This release adds a new [--encrypted | --no-encrypted] field in restore-from-cluster-snapshot API. Customers can now restore an unencrypted snapshot to a cluster encrypted with AWS Managed Key or their own KMS key.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.23.0](service/ssm/CHANGELOG.md#v1230-2022-03-23)
  * **Feature**: Update AddTagsToResource, ListTagsForResource, and RemoveTagsFromResource APIs to reflect the support for tagging Automation resources. Includes other minor documentation updates.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.18.1](service/transfer/CHANGELOG.md#v1181-2022-03-23)
  * **Documentation**: Documentation updates for AWS Transfer Family to describe how to remove an associated workflow from a server.

# Release (2022-03-22)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.18.0](service/costexplorer/CHANGELOG.md#v1180-2022-03-22)
  * **Feature**: Added three new APIs to support tagging and resource-level authorization on Cost Explorer resources: TagResource, UntagResource, ListTagsForResource.  Added optional parameters to CreateCostCategoryDefinition, CreateAnomalySubscription and CreateAnomalyMonitor APIs to support Tag On Create.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.18.2](service/ecs/CHANGELOG.md#v1182-2022-03-22)
  * **Documentation**: Documentation only update to address tickets
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.16.0](service/lakeformation/CHANGELOG.md#v1160-2022-03-22)
  * **Feature**: The release fixes the incorrect permissions called out in the documentation - DESCRIBE_TAG, ASSOCIATE_TAG, DELETE_TAG, ALTER_TAG. This trebuchet release fixes the corresponding SDK and documentation.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.16.0](service/location/CHANGELOG.md#v1160-2022-03-22)
  * **Feature**: Amazon Location Service now includes a MaxResults parameter for GetDevicePositionHistory requests.
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.14.0](service/polly/CHANGELOG.md#v1140-2022-03-22)
  * **Feature**: Amazon Polly adds new Catalan voice - Arlet. Arlet is available as Neural voice only.

# Release (2022-03-21)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.8.0](service/chimesdkmeetings/CHANGELOG.md#v180-2022-03-21)
  * **Feature**: Add support for media replication to link multiple WebRTC media sessions together to reach larger and global audiences. Participants connected to a replica session can be granted access to join the primary session and can switch sessions with their existing WebRTC connection
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.17.0](service/ecr/CHANGELOG.md#v1170-2022-03-21)
  * **Feature**: This release includes a fix in the DescribeImageScanFindings paginated output.
* `github.com/aws/aws-sdk-go-v2/service/mediaconnect`: [v1.16.0](service/mediaconnect/CHANGELOG.md#v1160-2022-03-21)
  * **Feature**: This release adds support for selecting a maintenance window.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.21.0](service/quicksight/CHANGELOG.md#v1210-2022-03-21)
  * **Feature**: AWS QuickSight Service Features - Expand public API support for group management.
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.16.1](service/ram/CHANGELOG.md#v1161-2022-03-21)
  * **Documentation**: Document improvements to the RAM API operations and parameter descriptions.

# Release (2022-03-18)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.22.0](service/glue/CHANGELOG.md#v1220-2022-03-18)
  * **Feature**: Added 9 new APIs for AWS Glue Interactive Sessions: ListSessions, StopSession, CreateSession, GetSession, DeleteSession, RunStatement, GetStatement, ListStatements, CancelStatement

# Release (2022-03-16)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.17.0](service/acmpca/CHANGELOG.md#v1170-2022-03-16)
  * **Feature**: AWS Certificate Manager (ACM) Private Certificate Authority (CA) now supports customizable certificate subject names and extensions.
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.13.0](service/amplifybackend/CHANGELOG.md#v1130-2022-03-16)
  * **Feature**: Adding the ability to customize Cognito verification messages for email and SMS in CreateBackendAuth and UpdateBackendAuth. Adding deprecation documentation for ForgotPassword in CreateBackendAuth and UpdateBackendAuth
* `github.com/aws/aws-sdk-go-v2/service/billingconductor`: [v1.0.0](service/billingconductor/CHANGELOG.md#v100-2022-03-16)
  * **Release**: New AWS service client module
  * **Feature**: This is the initial SDK release for AWS Billing Conductor. The AWS Billing Conductor is a customizable billing service, allowing you to customize your billing data to match your desired business structure.
* `github.com/aws/aws-sdk-go-v2/service/s3outposts`: [v1.13.0](service/s3outposts/CHANGELOG.md#v1130-2022-03-16)
  * **Feature**: S3 on Outposts is releasing a new API, ListSharedEndpoints, that lists all endpoints associated with S3 on Outpost, that has been shared by Resource Access Manager (RAM).
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.13.0](service/ssmincidents/CHANGELOG.md#v1130-2022-03-16)
  * **Feature**: Removed incorrect validation pattern for IncidentRecordSource.invokedBy

# Release (2022-03-15)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.15.0](service/cognitoidentityprovider/CHANGELOG.md#v1150-2022-03-15)
  * **Feature**: Updated EmailConfigurationType and SmsConfigurationType to reflect that you can now choose Amazon SES and Amazon SNS resources in the same Region.
* `github.com/aws/aws-sdk-go-v2/service/dataexchange`: [v1.15.0](service/dataexchange/CHANGELOG.md#v1150-2022-03-15)
  * **Feature**: This feature enables data providers to use the RevokeRevision operation to revoke subscriber access to a given revision. Subscribers are unable to interact with assets within a revoked revision.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.32.0](service/ec2/CHANGELOG.md#v1320-2022-03-15)
  * **Feature**: Adds the Cascade parameter to the DeleteIpam API. Customers can use this parameter to automatically delete their IPAM, including non-default scopes, pools, cidrs, and allocations. There mustn't be any pools provisioned in the default public scope to use this parameter.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.18.1](service/ecs/CHANGELOG.md#v1181-2022-03-15)
  * **Documentation**: Documentation only update to address tickets
* `github.com/aws/aws-sdk-go-v2/service/keyspaces`: [v1.0.2](service/keyspaces/CHANGELOG.md#v102-2022-03-15)
  * **Documentation**: Fixing formatting issues in CLI and SDK documentation
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.15.1](service/location/CHANGELOG.md#v1151-2022-03-15)
  * **Documentation**: New HERE style "VectorHereExplore" and "VectorHereExploreTruck".
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.18.1](service/rds/CHANGELOG.md#v1181-2022-03-15)
  * **Documentation**: Various documentation improvements
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.17.0](service/robomaker/CHANGELOG.md#v1170-2022-03-15)
  * **Feature**: This release deprecates ROS, Ubuntu and Gazbeo from RoboMaker Simulation Service Software Suites in favor of user-supplied containers and Relaxed Software Suites.

# Release (2022-03-14)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.19.0](service/configservice/CHANGELOG.md#v1190-2022-03-14)
  * **Feature**: Add resourceType enums for AWS::ECR::PublicRepository and AWS::EC2::LaunchTemplate
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.20.1](service/elasticache/CHANGELOG.md#v1201-2022-03-14)
  * **Documentation**: Doc only update for ElastiCache
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.23.0](service/kendra/CHANGELOG.md#v1230-2022-03-14)
  * **Feature**: Amazon Kendra now provides a data source connector for Slack. For more information, see https://docs.aws.amazon.com/kendra/latest/dg/data-source-slack.html
* `github.com/aws/aws-sdk-go-v2/service/timestreamquery`: [v1.14.0](service/timestreamquery/CHANGELOG.md#v1140-2022-03-14)
  * **Feature**: Amazon Timestream Scheduled Queries now support Timestamp datatype in a multi-measure record.

# Release (2022-03-11)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.20.0](service/chime/CHANGELOG.md#v1200-2022-03-11)
  * **Feature**: Chime VoiceConnector Logging APIs will now support MediaMetricLogs. Also CreateMeetingDialOut now returns AccessDeniedException.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.20.0](service/connect/CHANGELOG.md#v1200-2022-03-11)
  * **Feature**: This release adds support for enabling Rich Messaging when starting a new chat session via the StartChatContact API. Rich Messaging enables the following formatting options: bold, italics, hyperlinks, bulleted lists, and numbered lists.
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.20.0](service/lambda/CHANGELOG.md#v1200-2022-03-11)
  * **Feature**: Adds PrincipalOrgID support to AddPermission API. Customers can use it to manage permissions to lambda functions at AWS Organizations level.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.18.0](service/outposts/CHANGELOG.md#v1180-2022-03-11)
  * **Feature**: This release adds address filters for listSites
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.15.1](service/secretsmanager/CHANGELOG.md#v1151-2022-03-11)
  * **Documentation**: Documentation updates for Secrets Manager.
* `github.com/aws/aws-sdk-go-v2/service/transcribestreaming`: [v1.6.0](service/transcribestreaming/CHANGELOG.md#v160-2022-03-11)
  * **Feature**: Amazon Transcribe StartTranscription API now supports additional parameters for Language Identification feature: customVocabularies and customFilterVocabularies

# Release (2022-03-10)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.20.0](service/lexmodelsv2/CHANGELOG.md#v1200-2022-03-10)
  * **Feature**: This release makes slotTypeId an optional parameter in CreateSlot and UpdateSlot APIs in Amazon Lex V2 for model building. Customers can create and update slots without specifying a slot type id.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.18.0](service/transcribe/CHANGELOG.md#v1180-2022-03-10)
  * **Feature**: Documentation fix for API `StartMedicalTranscriptionJobRequest`, now showing min sample rate as 16khz
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.18.0](service/transfer/CHANGELOG.md#v1180-2022-03-10)
  * **Feature**: Adding more descriptive error types for managed workflows

# Release (2022-03-09)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.17.0](service/comprehend/CHANGELOG.md#v1170-2022-03-09)
  * **Feature**: Amazon Comprehend now supports extracting the sentiment associated with entities such as brands, products and services from text documents.

# Release (2022-03-08.3)

* No change notes available for this release.

# Release (2022-03-08.2)

* No change notes available for this release.

# Release (2022-03-08)

## General Highlights
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.11.0](service/amplify/CHANGELOG.md#v1110-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/amplifyuibuilder`: [v1.5.0](service/amplifyuibuilder/CHANGELOG.md#v150-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.14.0](service/appflow/CHANGELOG.md#v1140-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.11.0](service/apprunner/CHANGELOG.md#v1110-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.14.0](service/athena/CHANGELOG.md#v1140-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.15.0](service/braket/CHANGELOG.md#v1150-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.7.0](service/chimesdkmeetings/CHANGELOG.md#v170-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.15.0](service/cloudtrail/CHANGELOG.md#v1150-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.19.0](service/connect/CHANGELOG.md#v1190-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.16.0](service/devopsguru/CHANGELOG.md#v1160-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.31.0](service/ec2/CHANGELOG.md#v1310-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.16.0](service/ecr/CHANGELOG.md#v1160-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.18.0](service/ecs/CHANGELOG.md#v1180-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.20.0](service/elasticache/CHANGELOG.md#v1200-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.10.0](service/finspacedata/CHANGELOG.md#v1100-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/fis`: [v1.12.0](service/fis/CHANGELOG.md#v1120-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.20.0](service/fsx/CHANGELOG.md#v1200-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.14.0](service/gamelift/CHANGELOG.md#v1140-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.15.0](service/greengrassv2/CHANGELOG.md#v1150-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/internal/checksum`: [v1.1.0](service/internal/checksum/CHANGELOG.md#v110-2022-03-08)
  * **Feature**:  Updates the SDK's checksum validation logic to require opt-in to output response payload validation. The SDK was always preforming output response payload checksum validation, not respecting the output validation model option. Fixes [#1606](https://github.com/aws/aws-sdk-go-v2/issues/1606)
* `github.com/aws/aws-sdk-go-v2/service/kafkaconnect`: [v1.8.0](service/kafkaconnect/CHANGELOG.md#v180-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.22.0](service/kendra/CHANGELOG.md#v1220-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/keyspaces`: [v1.0.0](service/keyspaces/CHANGELOG.md#v100-2022-03-08)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/macie`: [v1.14.0](service/macie/CHANGELOG.md#v1140-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.15.0](service/mediapackage/CHANGELOG.md#v1150-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.13.0](service/mgn/CHANGELOG.md#v1130-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/migrationhubrefactorspaces`: [v1.5.0](service/migrationhubrefactorspaces/CHANGELOG.md#v150-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/mq`: [v1.12.0](service/mq/CHANGELOG.md#v1120-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/panorama`: [v1.6.0](service/panorama/CHANGELOG.md#v160-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.18.0](service/rds/CHANGELOG.md#v1180-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycluster`: [v1.8.0](service/route53recoverycluster/CHANGELOG.md#v180-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/servicecatalogappregistry`: [v1.12.0](service/servicecatalogappregistry/CHANGELOG.md#v1120-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.18.0](service/sqs/CHANGELOG.md#v1180-2022-03-08)
  * **Feature**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.16.0](service/sts/CHANGELOG.md#v1160-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.14.0](service/synthetics/CHANGELOG.md#v1140-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/timestreamquery`: [v1.13.0](service/timestreamquery/CHANGELOG.md#v1130-2022-03-08)
  * **Documentation**: Updated service client model to latest release.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.17.0](service/transfer/CHANGELOG.md#v1170-2022-03-08)
  * **Feature**: Updated service client model to latest release.

# Release (2022-02-24.2)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.21.0](service/autoscaling/CHANGELOG.md#v1210-2022-02-242)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.18.0](service/databrew/CHANGELOG.md#v1180-2022-02-242)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.15.0](service/fms/CHANGELOG.md#v1150-2022-02-242)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.17.0](service/lightsail/CHANGELOG.md#v1170-2022-02-242)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.19.0](service/route53/CHANGELOG.md#v1190-2022-02-242)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.20.0](service/s3control/CHANGELOG.md#v1200-2022-02-242)
  * **Feature**: API client updated

# Release (2022-02-24)

## General Highlights
* **Feature**: Adds RetryMaxAttempts and RetryMod to API client Options. This allows the API clients' default Retryer to be configured from the shared configuration files or environment variables. Adding a new Retry mode of `Adaptive`. `Adaptive` retry mode is an experimental mode, adding client rate limiting when throttles reponses are received from an API. See [retry.AdaptiveMode](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/retry#AdaptiveMode) for more details, and configuration options.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Bug Fix**: Fixes the AWS Sigv4 signer to trim header value's whitespace when computing the canonical headers block of the string to sign.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.14.0
  * **Feature**: Add new AdaptiveMode retryer to aws/retry package. This new retryer uses dynamic token bucketing with client ratelimiting when throttle responses are received.
  * **Feature**: Adds new interface aws.RetryerV2, replacing aws.Retryer and deprecating the GetInitialToken method in favor of GetAttemptToken so Context can be provided. The SDK will use aws.RetryerV2 internally. Wrapping aws.Retryers as aws.RetryerV2 automatically.
* `github.com/aws/aws-sdk-go-v2/config`: [v1.14.0](config/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: Adds support for loading RetryMaxAttempts and RetryMod from the environment and shared configuration files. These parameters drive how the SDK's API client will initialize its default retryer, if custome retryer has not been specified. See [config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) module and [aws.Config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#Config) for more information about and how to use these new options.
  * **Feature**: Adds support for the `ca_bundle` parameter in shared config and credentials files. The usage of the file is the same as environment variable, `AWS_CA_BUNDLE`, but sourced from shared config. Fixes [#1589](https://github.com/aws/aws-sdk-go-v2/issues/1589)
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.9.0](credentials/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: Adds support for `SourceIdentity` to `stscreds.AssumeRoleProvider` [#1588](https://github.com/aws/aws-sdk-go-v2/pull/1588). Fixes [#1575](https://github.com/aws/aws-sdk-go-v2/issues/1575)
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.7.0](feature/dynamodb/attributevalue/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: Fixes [#645](https://github.com/aws/aws-sdk-go-v2/issues/645), [#411](https://github.com/aws/aws-sdk-go-v2/issues/411) by adding support for (un)marshaling AttributeValue maps to Go maps key types of string, number, bool, and types implementing encoding.Text(un)Marshaler interface
  * **Bug Fix**: Fixes [#1569](https://github.com/aws/aws-sdk-go-v2/issues/1569) inconsistent serialization of Go struct field names
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression`: [v1.4.0](feature/dynamodb/expression/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: Add support for expression names with dots via new NameBuilder function NameNoDotSplit, related to [aws/aws-sdk-go#2570](https://github.com/aws/aws-sdk-go/issues/2570)
* `github.com/aws/aws-sdk-go-v2/feature/dynamodbstreams/attributevalue`: [v1.7.0](feature/dynamodbstreams/attributevalue/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: Fixes [#645](https://github.com/aws/aws-sdk-go-v2/issues/645), [#411](https://github.com/aws/aws-sdk-go-v2/issues/411) by adding support for (un)marshaling AttributeValue maps to Go maps key types of string, number, bool, and types implementing encoding.Text(un)Marshaler interface
  * **Bug Fix**: Fixes [#1569](https://github.com/aws/aws-sdk-go-v2/issues/1569) inconsistent serialization of Go struct field names
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.14.0](service/accessanalyzer/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/account`: [v1.5.0](service/account/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/acm`: [v1.13.0](service/acm/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.15.0](service/acmpca/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/alexaforbusiness`: [v1.13.0](service/alexaforbusiness/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.13.0](service/amp/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.10.0](service/amplify/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.11.0](service/amplifybackend/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amplifyuibuilder`: [v1.4.0](service/amplifyuibuilder/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.14.0](service/apigateway/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi`: [v1.9.0](service/apigatewaymanagementapi/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apigatewayv2`: [v1.11.0](service/apigatewayv2/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appconfig`: [v1.11.0](service/appconfig/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appconfigdata`: [v1.3.0](service/appconfigdata/CHANGELOG.md#v130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.13.0](service/appflow/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appintegrations`: [v1.12.0](service/appintegrations/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationautoscaling`: [v1.14.0](service/applicationautoscaling/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationcostprofiler`: [v1.8.0](service/applicationcostprofiler/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationdiscoveryservice`: [v1.11.0](service/applicationdiscoveryservice/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.14.0](service/applicationinsights/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.12.0](service/appmesh/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.10.0](service/apprunner/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.14.0](service/appstream/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.13.0](service/appsync/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.13.0](service/athena/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.16.0](service/auditmanager/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.20.0](service/autoscaling/CHANGELOG.md#v1200-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscalingplans`: [v1.11.0](service/autoscalingplans/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.14.0](service/backup/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/backupgateway`: [v1.4.0](service/backupgateway/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.15.0](service/batch/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.14.0](service/braket/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/budgets`: [v1.11.0](service/budgets/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.18.0](service/chime/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkidentity`: [v1.8.0](service/chimesdkidentity/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.6.0](service/chimesdkmeetings/CHANGELOG.md#v160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.8.0](service/chimesdkmessaging/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloud9`: [v1.15.0](service/cloud9/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudcontrol`: [v1.7.0](service/cloudcontrol/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/clouddirectory`: [v1.11.0](service/clouddirectory/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.19.0](service/cloudformation/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.15.0](service/cloudfront/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudhsm`: [v1.11.0](service/cloudhsm/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudhsmv2`: [v1.12.0](service/cloudhsmv2/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudsearch`: [v1.12.0](service/cloudsearch/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudsearchdomain`: [v1.10.0](service/cloudsearchdomain/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.14.0](service/cloudtrail/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.16.0](service/cloudwatch/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchevents`: [v1.13.0](service/cloudwatchevents/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.14.0](service/cloudwatchlogs/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codeartifact`: [v1.11.0](service/codeartifact/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.18.0](service/codebuild/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codecommit`: [v1.12.0](service/codecommit/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codedeploy`: [v1.13.0](service/codedeploy/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codeguruprofiler`: [v1.11.0](service/codeguruprofiler/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codegurureviewer`: [v1.14.0](service/codegurureviewer/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codepipeline`: [v1.12.0](service/codepipeline/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codestar`: [v1.10.0](service/codestar/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codestarconnections`: [v1.12.0](service/codestarconnections/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codestarnotifications`: [v1.10.0](service/codestarnotifications/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentity`: [v1.12.0](service/cognitoidentity/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.13.0](service/cognitoidentityprovider/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cognitosync`: [v1.10.0](service/cognitosync/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.15.0](service/comprehend/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/comprehendmedical`: [v1.12.0](service/comprehendmedical/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.16.0](service/computeoptimizer/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.17.0](service/configservice/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.18.0](service/connect/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connectcontactlens`: [v1.11.0](service/connectcontactlens/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connectparticipant`: [v1.10.0](service/connectparticipant/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/costandusagereportservice`: [v1.12.0](service/costandusagereportservice/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.16.0](service/costexplorer/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.16.0](service/customerprofiles/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.17.0](service/databasemigrationservice/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.17.0](service/databrew/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dataexchange`: [v1.13.0](service/dataexchange/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/datapipeline`: [v1.12.0](service/datapipeline/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.13.0](service/datasync/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dax`: [v1.10.0](service/dax/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/detective`: [v1.14.0](service/detective/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/devicefarm`: [v1.12.0](service/devicefarm/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.15.0](service/devopsguru/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.16.0](service/directconnect/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directoryservice`: [v1.12.0](service/directoryservice/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dlm`: [v1.10.0](service/dlm/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.16.0](service/docdb/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/drs`: [v1.4.0](service/drs/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.14.0](service/dynamodb/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodbstreams`: [v1.12.0](service/dynamodbstreams/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.13.0](service/ebs/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.30.0](service/ec2/CHANGELOG.md#v1300-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect`: [v1.12.0](service/ec2instanceconnect/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.15.0](service/ecr/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecrpublic`: [v1.12.0](service/ecrpublic/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.17.0](service/ecs/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.15.0](service/efs/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.19.0](service/eks/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.19.0](service/elasticache/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk`: [v1.13.0](service/elasticbeanstalk/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticinference`: [v1.10.0](service/elasticinference/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing`: [v1.13.0](service/elasticloadbalancing/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.17.0](service/elasticloadbalancingv2/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.14.0](service/elasticsearchservice/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elastictranscoder`: [v1.12.0](service/elastictranscoder/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.16.0](service/emr/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emrcontainers`: [v1.12.0](service/emrcontainers/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.14.0](service/eventbridge/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.5.0](service/evidently/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/finspace`: [v1.7.0](service/finspace/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.9.0](service/finspacedata/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/firehose`: [v1.13.0](service/firehose/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fis`: [v1.11.0](service/fis/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.14.0](service/fms/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.18.0](service/forecast/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecastquery`: [v1.10.0](service/forecastquery/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
  * **Bug Fix**: Fixed an issue that resulted in the wrong service endpoints being constructed.
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.18.0](service/frauddetector/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.19.0](service/fsx/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.13.0](service/gamelift/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glacier`: [v1.12.0](service/glacier/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/globalaccelerator`: [v1.12.0](service/globalaccelerator/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.20.0](service/glue/CHANGELOG.md#v1200-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/grafana`: [v1.6.0](service/grafana/CHANGELOG.md#v160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/greengrass`: [v1.12.0](service/greengrass/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.14.0](service/greengrassv2/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/groundstation`: [v1.12.0](service/groundstation/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.12.0](service/guardduty/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.14.0](service/health/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/healthlake`: [v1.13.0](service/healthlake/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/honeycode`: [v1.11.0](service/honeycode/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.17.0](service/iam/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.13.0](service/identitystore/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.18.0](service/imagebuilder/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/inspector`: [v1.11.0](service/inspector/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/inspector2`: [v1.5.0](service/inspector2/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/internal/checksum`: [v1.0.0](service/internal/checksum/CHANGELOG.md#v100-2022-02-24)
  * **Release**: New module for computing checksums
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.22.0](service/iot/CHANGELOG.md#v1220-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot1clickdevicesservice`: [v1.9.0](service/iot1clickdevicesservice/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot1clickprojects`: [v1.10.0](service/iot1clickprojects/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotanalytics`: [v1.11.0](service/iotanalytics/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotdataplane`: [v1.10.0](service/iotdataplane/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotdeviceadvisor`: [v1.13.0](service/iotdeviceadvisor/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotevents`: [v1.13.0](service/iotevents/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ioteventsdata`: [v1.10.0](service/ioteventsdata/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotfleethub`: [v1.11.0](service/iotfleethub/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotjobsdataplane`: [v1.10.0](service/iotjobsdataplane/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotsecuretunneling`: [v1.11.0](service/iotsecuretunneling/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.19.0](service/iotsitewise/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotthingsgraph`: [v1.11.0](service/iotthingsgraph/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iottwinmaker`: [v1.4.0](service/iottwinmaker/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.17.0](service/iotwireless/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.15.0](service/ivs/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.16.0](service/kafka/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kafkaconnect`: [v1.7.0](service/kafkaconnect/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.21.0](service/kendra/CHANGELOG.md#v1210-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.14.0](service/kinesis/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalytics`: [v1.12.0](service/kinesisanalytics/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalyticsv2`: [v1.13.0](service/kinesisanalyticsv2/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideo`: [v1.10.0](service/kinesisvideo/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideoarchivedmedia`: [v1.11.0](service/kinesisvideoarchivedmedia/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideomedia`: [v1.9.0](service/kinesisvideomedia/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisvideosignaling`: [v1.9.0](service/kinesisvideosignaling/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.15.0](service/kms/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.14.0](service/lakeformation/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.18.0](service/lambda/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelbuildingservice`: [v1.15.0](service/lexmodelbuildingservice/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.18.0](service/lexmodelsv2/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimeservice`: [v1.11.0](service/lexruntimeservice/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.13.0](service/lexruntimev2/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.14.0](service/licensemanager/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.16.0](service/lightsail/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.14.0](service/location/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.11.0](service/lookoutequipment/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.10.0](service/lookoutmetrics/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutvision`: [v1.11.0](service/lookoutvision/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/machinelearning`: [v1.13.0](service/machinelearning/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie`: [v1.13.0](service/macie/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.19.0](service/macie2/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/managedblockchain`: [v1.11.0](service/managedblockchain/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/marketplacecatalog`: [v1.11.0](service/marketplacecatalog/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/marketplacecommerceanalytics`: [v1.10.0](service/marketplacecommerceanalytics/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/marketplaceentitlementservice`: [v1.10.0](service/marketplaceentitlementservice/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/marketplacemetering`: [v1.12.0](service/marketplacemetering/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconnect`: [v1.14.0](service/mediaconnect/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.20.0](service/mediaconvert/CHANGELOG.md#v1200-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.18.0](service/medialive/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.14.0](service/mediapackage/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.15.0](service/mediapackagevod/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediastore`: [v1.11.0](service/mediastore/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediastoredata`: [v1.11.0](service/mediastoredata/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.15.0](service/mediatailor/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/memorydb`: [v1.8.0](service/memorydb/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.12.0](service/mgn/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhub`: [v1.11.0](service/migrationhub/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhubconfig`: [v1.11.0](service/migrationhubconfig/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhubrefactorspaces`: [v1.4.0](service/migrationhubrefactorspaces/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhubstrategy`: [v1.4.0](service/migrationhubstrategy/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mobile`: [v1.10.0](service/mobile/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mq`: [v1.11.0](service/mq/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mturk`: [v1.12.0](service/mturk/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mwaa`: [v1.11.0](service/mwaa/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.15.0](service/neptune/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.14.0](service/networkfirewall/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.11.0](service/networkmanager/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.11.0](service/nimble/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.8.0](service/opensearch/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opsworks`: [v1.12.0](service/opsworks/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opsworkscm`: [v1.13.0](service/opsworkscm/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/organizations`: [v1.13.0](service/organizations/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.16.0](service/outposts/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/panorama`: [v1.5.0](service/panorama/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.16.0](service/personalize/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalizeevents`: [v1.10.0](service/personalizeevents/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalizeruntime`: [v1.10.0](service/personalizeruntime/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.13.0](service/pi/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.15.0](service/pinpoint/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pinpointemail`: [v1.10.0](service/pinpointemail/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoice`: [v1.9.0](service/pinpointsmsvoice/CHANGELOG.md#v190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.12.0](service/polly/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pricing`: [v1.13.0](service/pricing/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.11.0](service/proton/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.13.0](service/qldb/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/qldbsession`: [v1.12.0](service/qldbsession/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.19.0](service/quicksight/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.15.0](service/ram/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rbin`: [v1.5.0](service/rbin/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.17.0](service/rds/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rdsdata`: [v1.10.0](service/rdsdata/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.20.0](service/redshift/CHANGELOG.md#v1200-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshiftdata`: [v1.14.0](service/redshiftdata/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.16.0](service/rekognition/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/resiliencehub`: [v1.4.0](service/resiliencehub/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/resourcegroups`: [v1.11.0](service/resourcegroups/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi`: [v1.12.0](service/resourcegroupstaggingapi/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.15.0](service/robomaker/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.18.0](service/route53/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53domains`: [v1.11.0](service/route53domains/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycluster`: [v1.7.0](service/route53recoverycluster/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycontrolconfig`: [v1.8.0](service/route53recoverycontrolconfig/CHANGELOG.md#v180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53recoveryreadiness`: [v1.7.0](service/route53recoveryreadiness/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53resolver`: [v1.14.0](service/route53resolver/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rum`: [v1.5.0](service/rum/CHANGELOG.md#v150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.25.0](service/s3/CHANGELOG.md#v1250-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.19.0](service/s3control/CHANGELOG.md#v1190-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3outposts`: [v1.11.0](service/s3outposts/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.25.0](service/sagemaker/CHANGELOG.md#v1250-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakera2iruntime`: [v1.11.0](service/sagemakera2iruntime/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakeredge`: [v1.10.0](service/sagemakeredge/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerfeaturestoreruntime`: [v1.10.0](service/sagemakerfeaturestoreruntime/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime`: [v1.14.0](service/sagemakerruntime/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/savingsplans`: [v1.10.0](service/savingsplans/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/schemas`: [v1.13.0](service/schemas/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.14.0](service/secretsmanager/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.18.0](service/securityhub/CHANGELOG.md#v1180-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/serverlessapplicationrepository`: [v1.10.0](service/serverlessapplicationrepository/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicecatalog`: [v1.12.0](service/servicecatalog/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicecatalogappregistry`: [v1.11.0](service/servicecatalogappregistry/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicediscovery`: [v1.16.0](service/servicediscovery/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicequotas`: [v1.12.0](service/servicequotas/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ses`: [v1.13.0](service/ses/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sesv2`: [v1.12.0](service/sesv2/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sfn`: [v1.12.0](service/sfn/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/shield`: [v1.15.0](service/shield/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/signer`: [v1.12.0](service/signer/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sms`: [v1.11.0](service/sms/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowball`: [v1.14.0](service/snowball/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowdevicemanagement`: [v1.7.0](service/snowdevicemanagement/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.16.0](service/sns/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.17.0](service/sqs/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.21.0](service/ssm/CHANGELOG.md#v1210-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.12.0](service/ssmcontacts/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.11.0](service/ssmincidents/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.10.0](service/sso/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.13.0](service/ssoadmin/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.11.0](service/ssooidc/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.15.0](service/storagegateway/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.15.0](service/sts/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/support`: [v1.12.0](service/support/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/swf`: [v1.12.0](service/swf/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.13.0](service/synthetics/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.13.0](service/textract/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/timestreamquery`: [v1.12.0](service/timestreamquery/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/timestreamwrite`: [v1.12.0](service/timestreamwrite/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.16.0](service/transcribe/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transcribestreaming`: [v1.4.0](service/transcribestreaming/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.16.0](service/transfer/CHANGELOG.md#v1160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/translate`: [v1.12.0](service/translate/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/voiceid`: [v1.7.0](service/voiceid/CHANGELOG.md#v170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/waf`: [v1.10.0](service/waf/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafregional`: [v1.11.0](service/wafregional/CHANGELOG.md#v1110-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.17.0](service/wafv2/CHANGELOG.md#v1170-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wellarchitected`: [v1.13.0](service/wellarchitected/CHANGELOG.md#v1130-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.6.0](service/wisdom/CHANGELOG.md#v160-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workdocs`: [v1.10.0](service/workdocs/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/worklink`: [v1.10.0](service/worklink/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.14.0](service/workmail/CHANGELOG.md#v1140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmailmessageflow`: [v1.10.0](service/workmailmessageflow/CHANGELOG.md#v1100-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.15.0](service/workspaces/CHANGELOG.md#v1150-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspacesweb`: [v1.4.0](service/workspacesweb/CHANGELOG.md#v140-2022-02-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/xray`: [v1.12.0](service/xray/CHANGELOG.md#v1120-2022-02-24)
  * **Feature**: API client updated

# Release (2022-01-28)

## General Highlights
* **Bug Fix**: Fixes the SDK's handling of `duration_sections` in the shared credentials file or specified in multiple shared config and shared credentials files under the same profile. [#1568](https://github.com/aws/aws-sdk-go-v2/pull/1568). Thanks to [Amir Szekely](https://github.com/kichik) for help reproduce this bug.
* **Bug Fix**: Updates SDK API client deserialization to pre-allocate byte slice and string response payloads, [#1565](https://github.com/aws/aws-sdk-go-v2/pull/1565). Thanks to [Tyson Mote](https://github.com/tysonmote) for submitting this PR.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.13.1](config/CHANGELOG.md#v1131-2022-01-28)
  * **Bug Fix**: Fixes LoadDefaultConfig handling of errors returned by passed in functional options. Previously errors returned from the LoadOptions passed into LoadDefaultConfig were incorrectly ignored. [#1562](https://github.com/aws/aws-sdk-go-v2/pull/1562). Thanks to [Pinglei Guo](https://github.com/pingleig) for submitting this PR.
  * **Bug Fix**: Updates `config` module to use os.UserHomeDir instead of hard coded environment variable for OS. [#1563](https://github.com/aws/aws-sdk-go-v2/pull/1563)
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.13.0](service/applicationinsights/CHANGELOG.md#v1130-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.13.1](service/cloudtrail/CHANGELOG.md#v1131-2022-01-28)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/codegurureviewer`: [v1.13.1](service/codegurureviewer/CHANGELOG.md#v1131-2022-01-28)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.16.0](service/configservice/CHANGELOG.md#v1160-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.17.0](service/connect/CHANGELOG.md#v1170-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.12.1](service/ebs/CHANGELOG.md#v1121-2022-01-28)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.29.0](service/ec2/CHANGELOG.md#v1290-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect`: [v1.11.0](service/ec2instanceconnect/CHANGELOG.md#v1110-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.14.0](service/efs/CHANGELOG.md#v1140-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/fis`: [v1.10.0](service/fis/CHANGELOG.md#v1100-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.17.0](service/frauddetector/CHANGELOG.md#v1170-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.18.0](service/fsx/CHANGELOG.md#v1180-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/greengrass`: [v1.11.0](service/greengrass/CHANGELOG.md#v1110-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.13.0](service/greengrassv2/CHANGELOG.md#v1130-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.11.0](service/guardduty/CHANGELOG.md#v1110-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/honeycode`: [v1.10.0](service/honeycode/CHANGELOG.md#v1100-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.14.0](service/ivs/CHANGELOG.md#v1140-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.15.0](service/kafka/CHANGELOG.md#v1150-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.13.0](service/location/CHANGELOG.md#v1130-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.9.0](service/lookoutmetrics/CHANGELOG.md#v190-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.18.0](service/macie2/CHANGELOG.md#v1180-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.19.0](service/mediaconvert/CHANGELOG.md#v1190-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.14.0](service/mediatailor/CHANGELOG.md#v1140-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.14.0](service/ram/CHANGELOG.md#v1140-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53recoveryreadiness`: [v1.6.1](service/route53recoveryreadiness/CHANGELOG.md#v161-2022-01-28)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.24.0](service/sagemaker/CHANGELOG.md#v1240-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.17.0](service/securityhub/CHANGELOG.md#v1170-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.14.0](service/storagegateway/CHANGELOG.md#v1140-2022-01-28)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.15.0](service/transcribe/CHANGELOG.md#v1150-2022-01-28)
  * **Feature**: Updated to latest API model.

# Release (2022-01-14)

## General Highlights
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.13.0
  * **Bug Fix**: Updates the Retry middleware to release the retry token, on subsequent attempts. This fixes #1413, and is based on PR #1424
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.6.0](feature/dynamodb/attributevalue/CHANGELOG.md#v160-2022-01-14)
  * **Feature**: Adds new MarshalWithOptions and UnmarshalWithOptions helpers allowing Encoding and Decoding options to be specified when serializing AttributeValues. Addresses issue: https://github.com/aws/aws-sdk-go-v2/issues/1494
* `github.com/aws/aws-sdk-go-v2/feature/dynamodbstreams/attributevalue`: [v1.6.0](feature/dynamodbstreams/attributevalue/CHANGELOG.md#v160-2022-01-14)
  * **Feature**: Adds new MarshalWithOptions and UnmarshalWithOptions helpers allowing Encoding and Decoding options to be specified when serializing AttributeValues. Addresses issue: https://github.com/aws/aws-sdk-go-v2/issues/1494
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.12.0](service/appsync/CHANGELOG.md#v1120-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/autoscalingplans`: [v1.10.0](service/autoscalingplans/CHANGELOG.md#v1100-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.15.0](service/computeoptimizer/CHANGELOG.md#v1150-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.15.0](service/costexplorer/CHANGELOG.md#v1150-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.16.0](service/databasemigrationservice/CHANGELOG.md#v1160-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.16.0](service/databrew/CHANGELOG.md#v1160-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.28.0](service/ec2/CHANGELOG.md#v1280-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.18.0](service/elasticache/CHANGELOG.md#v1180-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.13.0](service/elasticsearchservice/CHANGELOG.md#v1130-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.8.0](service/finspacedata/CHANGELOG.md#v180-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.13.0](service/fms/CHANGELOG.md#v1130-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.19.0](service/glue/CHANGELOG.md#v1190-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/honeycode`: [v1.9.0](service/honeycode/CHANGELOG.md#v190-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.12.0](service/identitystore/CHANGELOG.md#v1120-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/ioteventsdata`: [v1.9.0](service/ioteventsdata/CHANGELOG.md#v190-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.16.0](service/iotwireless/CHANGELOG.md#v1160-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.20.0](service/kendra/CHANGELOG.md#v1200-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.17.0](service/lexmodelsv2/CHANGELOG.md#v1170-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.12.0](service/lexruntimev2/CHANGELOG.md#v1120-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.8.0](service/lookoutmetrics/CHANGELOG.md#v180-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.17.0](service/medialive/CHANGELOG.md#v1170-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.13.0](service/mediatailor/CHANGELOG.md#v1130-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/mwaa`: [v1.10.0](service/mwaa/CHANGELOG.md#v1100-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.10.0](service/nimble/CHANGELOG.md#v1100-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.7.0](service/opensearch/CHANGELOG.md#v170-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.12.0](service/pi/CHANGELOG.md#v1120-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.14.0](service/pinpoint/CHANGELOG.md#v1140-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.16.0](service/rds/CHANGELOG.md#v1160-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.20.0](service/ssm/CHANGELOG.md#v1200-2022-01-14)
  * **Feature**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.9.0](service/sso/CHANGELOG.md#v190-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.14.0](service/transcribe/CHANGELOG.md#v1140-2022-01-14)
  * **Documentation**: Updated API models
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.14.0](service/workspaces/CHANGELOG.md#v1140-2022-01-14)
  * **Feature**: Updated API models

# Release (2022-01-07)

## General Highlights
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.12.0](config/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: Add load option for CredentialCache. Adds a new member to the LoadOptions struct, CredentialsCacheOptions. This member allows specifying a function that will be used to configure the CredentialsCache. The CredentialsCacheOptions will only be used if the configuration loader will wrap the underlying credential provider in the CredentialsCache.
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.12.0](service/appstream/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.12.0](service/cloudtrail/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/detective`: [v1.12.0](service/detective/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.27.0](service/ec2/CHANGELOG.md#v1270-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.15.0](service/ecs/CHANGELOG.md#v1150-2022-01-07)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.17.0](service/eks/CHANGELOG.md#v1170-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.18.0](service/glue/CHANGELOG.md#v1180-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.11.0](service/greengrassv2/CHANGELOG.md#v1110-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.20.0](service/iot/CHANGELOG.md#v1200-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.12.0](service/lakeformation/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.16.0](service/lambda/CHANGELOG.md#v1160-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.17.0](service/mediaconvert/CHANGELOG.md#v1170-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.17.0](service/quicksight/CHANGELOG.md#v1170-2022-01-07)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.15.0](service/rds/CHANGELOG.md#v1150-2022-01-07)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.14.0](service/rekognition/CHANGELOG.md#v1140-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.23.0](service/s3/CHANGELOG.md#v1230-2022-01-07)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.17.0](service/s3control/CHANGELOG.md#v1170-2022-01-07)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3outposts`: [v1.9.0](service/s3outposts/CHANGELOG.md#v190-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.22.0](service/sagemaker/CHANGELOG.md#v1220-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.12.0](service/secretsmanager/CHANGELOG.md#v1120-2022-01-07)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.9.0](service/ssooidc/CHANGELOG.md#v190-2022-01-07)
  * **Feature**: API client updated

# Release (2021-12-21)

## General Highlights
* **Feature**: API Paginators now support specifying the initial starting token, and support stopping on empty string tokens.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.11.0](service/accessanalyzer/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/acm`: [v1.10.0](service/acm/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.11.0](service/apigateway/CHANGELOG.md#v1110-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationautoscaling`: [v1.11.0](service/applicationautoscaling/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.10.0](service/appsync/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.17.0](service/autoscaling/CHANGELOG.md#v1170-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.3.0](service/chimesdkmeetings/CHANGELOG.md#v130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.5.0](service/chimesdkmessaging/CHANGELOG.md#v150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudcontrol`: [v1.4.0](service/cloudcontrol/CHANGELOG.md#v140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.16.0](service/cloudformation/CHANGELOG.md#v1160-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.13.0](service/cloudwatch/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchevents`: [v1.10.0](service/cloudwatchevents/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.11.0](service/cloudwatchlogs/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: API client updated
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/codedeploy`: [v1.10.0](service/codedeploy/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/comprehendmedical`: [v1.9.0](service/comprehendmedical/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.13.0](service/configservice/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.13.0](service/customerprofiles/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.14.0](service/databasemigrationservice/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.10.0](service/datasync/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.12.0](service/devopsguru/CHANGELOG.md#v1120-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.13.0](service/directconnect/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.13.0](service/docdb/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.11.0](service/dynamodb/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/dynamodbstreams`: [v1.9.0](service/dynamodbstreams/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.26.0](service/ec2/CHANGELOG.md#v1260-2021-12-21)
  * **Feature**: API client updated
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.12.0](service/ecr/CHANGELOG.md#v1120-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.14.0](service/ecs/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.16.0](service/elasticache/CHANGELOG.md#v1160-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing`: [v1.10.0](service/elasticloadbalancing/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.14.0](service/elasticloadbalancingv2/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.11.0](service/elasticsearchservice/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.13.0](service/emr/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.11.0](service/eventbridge/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.6.0](service/finspacedata/CHANGELOG.md#v160-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.15.0](service/forecast/CHANGELOG.md#v1150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glacier`: [v1.9.0](service/glacier/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/groundstation`: [v1.9.0](service/groundstation/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.11.0](service/health/CHANGELOG.md#v1110-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.15.0](service/imagebuilder/CHANGELOG.md#v1150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.19.0](service/iot/CHANGELOG.md#v1190-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.11.0](service/kinesis/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalytics`: [v1.9.0](service/kinesisanalytics/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalyticsv2`: [v1.10.0](service/kinesisanalyticsv2/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.12.0](service/kms/CHANGELOG.md#v1120-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.15.0](service/lambda/CHANGELOG.md#v1150-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.15.0](service/lexmodelsv2/CHANGELOG.md#v1150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.10.0](service/location/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.6.0](service/lookoutmetrics/CHANGELOG.md#v160-2021-12-21)
  * **Feature**: API client updated
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/lookoutvision`: [v1.8.0](service/lookoutvision/CHANGELOG.md#v180-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/marketplacemetering`: [v1.9.0](service/marketplacemetering/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/mediaconnect`: [v1.11.0](service/mediaconnect/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.12.0](service/neptune/CHANGELOG.md#v1120-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.11.0](service/networkfirewall/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.8.0](service/nimble/CHANGELOG.md#v180-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.5.0](service/opensearch/CHANGELOG.md#v150-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.13.0](service/outposts/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.10.0](service/pi/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.10.0](service/qldb/CHANGELOG.md#v1100-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.14.0](service/rds/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.17.0](service/redshift/CHANGELOG.md#v1170-2021-12-21)
  * **Feature**: API client updated
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/resourcegroups`: [v1.8.0](service/resourcegroups/CHANGELOG.md#v180-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi`: [v1.9.0](service/resourcegroupstaggingapi/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.15.0](service/route53/CHANGELOG.md#v1150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53domains`: [v1.8.0](service/route53domains/CHANGELOG.md#v180-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycontrolconfig`: [v1.5.0](service/route53recoverycontrolconfig/CHANGELOG.md#v150-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.22.0](service/s3/CHANGELOG.md#v1220-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.16.0](service/s3control/CHANGELOG.md#v1160-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.21.0](service/sagemaker/CHANGELOG.md#v1210-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/savingsplans`: [v1.7.3](service/savingsplans/CHANGELOG.md#v173-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.11.0](service/secretsmanager/CHANGELOG.md#v1110-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.14.0](service/securityhub/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sfn`: [v1.9.0](service/sfn/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/sms`: [v1.8.0](service/sms/CHANGELOG.md#v180-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.13.0](service/sns/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.14.0](service/sqs/CHANGELOG.md#v1140-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.18.0](service/ssm/CHANGELOG.md#v1180-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.12.0](service/sts/CHANGELOG.md#v1120-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/support`: [v1.9.0](service/support/CHANGELOG.md#v190-2021-12-21)
  * **Documentation**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/swf`: [v1.9.0](service/swf/CHANGELOG.md#v190-2021-12-21)
  * **Feature**: Updated to latest service endpoints
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.13.0](service/transfer/CHANGELOG.md#v1130-2021-12-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.11.0](service/workmail/CHANGELOG.md#v1110-2021-12-21)
  * **Feature**: API client updated

# Release (2021-12-03)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.10.1](service/accessanalyzer/CHANGELOG.md#v1101-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.9.3](service/amp/CHANGELOG.md#v193-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/amplifyuibuilder`: [v1.0.0](service/amplifyuibuilder/CHANGELOG.md#v100-2021-12-03)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.8.3](service/appmesh/CHANGELOG.md#v183-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.10.2](service/braket/CHANGELOG.md#v1102-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/codeguruprofiler`: [v1.7.3](service/codeguruprofiler/CHANGELOG.md#v173-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.1.1](service/evidently/CHANGELOG.md#v111-2021-12-03)
  * **Bug Fix**: Fixed a bug that prevented the resolution of the correct endpoint for some API operations.
* `github.com/aws/aws-sdk-go-v2/service/grafana`: [v1.2.3](service/grafana/CHANGELOG.md#v123-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.9.2](service/location/CHANGELOG.md#v192-2021-12-03)
  * **Bug Fix**: Fixed a bug that prevented the resolution of the correct endpoint for some API operations.
  * **Bug Fix**: Fixed an issue that caused some operations to not be signed using sigv4, resulting in authentication failures.
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.7.0](service/networkmanager/CHANGELOG.md#v170-2021-12-03)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.7.3](service/nimble/CHANGELOG.md#v173-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.7.2](service/proton/CHANGELOG.md#v172-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.10.0](service/ram/CHANGELOG.md#v1100-2021-12-03)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.12.0](service/rekognition/CHANGELOG.md#v1120-2021-12-03)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowdevicemanagement`: [v1.3.3](service/snowdevicemanagement/CHANGELOG.md#v133-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.2.3](service/wisdom/CHANGELOG.md#v123-2021-12-03)
  * **Bug Fix**: Fixed an issue that prevent auto-filling of an API's idempotency parameters when not explictly provided by the caller.

# Release (2021-12-02)

## General Highlights
* **Bug Fix**: Fixes a bug that prevented aws.EndpointResolverWithOptions from being used by the service client. ([#1514](https://github.com/aws/aws-sdk-go-v2/pull/1514))
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.11.0](config/CHANGELOG.md#v1110-2021-12-02)
  * **Feature**: Add support for specifying `EndpointResolverWithOptions` on `LoadOptions`, and associated `WithEndpointResolverWithOptions`.
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.10.0](service/accessanalyzer/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.9.0](service/applicationinsights/CHANGELOG.md#v190-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/backupgateway`: [v1.0.0](service/backupgateway/CHANGELOG.md#v100-2021-12-02)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/cloudhsm`: [v1.8.0](service/cloudhsm/CHANGELOG.md#v180-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.11.0](service/devopsguru/CHANGELOG.md#v1110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.12.0](service/directconnect/CHANGELOG.md#v1120-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.10.0](service/dynamodb/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.25.0](service/ec2/CHANGELOG.md#v1250-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.1.0](service/evidently/CHANGELOG.md#v110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.14.0](service/fsx/CHANGELOG.md#v1140-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.16.0](service/glue/CHANGELOG.md#v1160-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/inspector2`: [v1.1.0](service/inspector2/CHANGELOG.md#v110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.18.0](service/iot/CHANGELOG.md#v1180-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iottwinmaker`: [v1.0.0](service/iottwinmaker/CHANGELOG.md#v100-2021-12-02)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.11.0](service/kafka/CHANGELOG.md#v1110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.17.0](service/kendra/CHANGELOG.md#v1170-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.10.0](service/kinesis/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.10.0](service/lakeformation/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.14.0](service/lexmodelsv2/CHANGELOG.md#v1140-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.10.0](service/lexruntimev2/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: Support has been added for the `StartConversation` API.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.12.0](service/outposts/CHANGELOG.md#v1120-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rbin`: [v1.1.0](service/rbin/CHANGELOG.md#v110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshiftdata`: [v1.10.0](service/redshiftdata/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rum`: [v1.1.0](service/rum/CHANGELOG.md#v110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.21.0](service/s3/CHANGELOG.md#v1210-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.20.0](service/sagemaker/CHANGELOG.md#v1200-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime`: [v1.11.0](service/sagemakerruntime/CHANGELOG.md#v1110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/shield`: [v1.11.0](service/shield/CHANGELOG.md#v1110-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowball`: [v1.10.0](service/snowball/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.10.0](service/storagegateway/CHANGELOG.md#v1100-2021-12-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspacesweb`: [v1.0.0](service/workspacesweb/CHANGELOG.md#v100-2021-12-02)
  * **Release**: New AWS service client module

# Release (2021-11-30)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.16.0](service/autoscaling/CHANGELOG.md#v1160-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.10.0](service/backup/CHANGELOG.md#v1100-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.10.0](service/braket/CHANGELOG.md#v1100-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.2.0](service/chimesdkmeetings/CHANGELOG.md#v120-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.15.0](service/cloudformation/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.13.0](service/computeoptimizer/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.13.0](service/connect/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.12.0](service/customerprofiles/CHANGELOG.md#v1120-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.13.0](service/databasemigrationservice/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dataexchange`: [v1.9.0](service/dataexchange/CHANGELOG.md#v190-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.9.0](service/dynamodb/CHANGELOG.md#v190-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.24.0](service/ec2/CHANGELOG.md#v1240-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.11.0](service/ecr/CHANGELOG.md#v1110-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.13.0](service/ecs/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.15.0](service/eks/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.15.0](service/elasticache/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.13.0](service/elasticloadbalancingv2/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.10.0](service/elasticsearchservice/CHANGELOG.md#v1100-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/evidently`: [v1.0.0](service/evidently/CHANGELOG.md#v100-2021-11-30)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.5.0](service/finspacedata/CHANGELOG.md#v150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.14.0](service/imagebuilder/CHANGELOG.md#v1140-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/inspector2`: [v1.0.0](service/inspector2/CHANGELOG.md#v100-2021-11-30)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery`: [v1.3.2](service/internal/endpoint-discovery/CHANGELOG.md#v132-2021-11-30)
  * **Bug Fix**: Fixed a race condition that caused concurrent calls relying on endpoint discovery to share the same `url.URL` reference in their operation's http.Request.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.17.0](service/iot/CHANGELOG.md#v1170-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotdeviceadvisor`: [v1.9.0](service/iotdeviceadvisor/CHANGELOG.md#v190-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.15.0](service/iotsitewise/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.13.0](service/iotwireless/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.14.0](service/lambda/CHANGELOG.md#v1140-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.14.0](service/macie2/CHANGELOG.md#v1140-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.8.0](service/mgn/CHANGELOG.md#v180-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhubrefactorspaces`: [v1.0.0](service/migrationhubrefactorspaces/CHANGELOG.md#v100-2021-11-30)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.4.0](service/opensearch/CHANGELOG.md#v140-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.11.0](service/outposts/CHANGELOG.md#v1110-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.12.0](service/personalize/CHANGELOG.md#v1120-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalizeruntime`: [v1.7.0](service/personalizeruntime/CHANGELOG.md#v170-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.12.0](service/pinpoint/CHANGELOG.md#v1120-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.7.0](service/proton/CHANGELOG.md#v170-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.15.0](service/quicksight/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rbin`: [v1.0.0](service/rbin/CHANGELOG.md#v100-2021-11-30)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.13.0](service/rds/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.16.0](service/redshift/CHANGELOG.md#v1160-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rum`: [v1.0.0](service/rum/CHANGELOG.md#v100-2021-11-30)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.20.0](service/s3/CHANGELOG.md#v1200-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.15.0](service/s3control/CHANGELOG.md#v1150-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.13.0](service/sqs/CHANGELOG.md#v1130-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.17.0](service/ssm/CHANGELOG.md#v1170-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.11.0](service/sts/CHANGELOG.md#v1110-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.10.0](service/textract/CHANGELOG.md#v1100-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/timestreamquery`: [v1.8.0](service/timestreamquery/CHANGELOG.md#v180-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/timestreamwrite`: [v1.8.0](service/timestreamwrite/CHANGELOG.md#v180-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transcribestreaming`: [v1.1.0](service/transcribestreaming/CHANGELOG.md#v110-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/translate`: [v1.8.0](service/translate/CHANGELOG.md#v180-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wellarchitected`: [v1.9.0](service/wellarchitected/CHANGELOG.md#v190-2021-11-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.11.0](service/workspaces/CHANGELOG.md#v1110-2021-11-30)
  * **Feature**: API client updated

# Release (2021-11-19)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.11.1
  * **Bug Fix**: Fixed a bug that prevented aws.EndpointResolverWithOptionsFunc from satisfying the aws.EndpointResolverWithOptions interface.
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.8.0](service/amplifybackend/CHANGELOG.md#v180-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.10.0](service/apigateway/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appconfig`: [v1.7.0](service/appconfig/CHANGELOG.md#v170-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appconfigdata`: [v1.0.0](service/appconfigdata/CHANGELOG.md#v100-2021-11-19)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.8.0](service/applicationinsights/CHANGELOG.md#v180-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.10.0](service/appstream/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.12.0](service/auditmanager/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.11.0](service/batch/CHANGELOG.md#v1110-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.14.0](service/chime/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.1.0](service/chimesdkmeetings/CHANGELOG.md#v110-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.14.0](service/cloudformation/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.10.0](service/cloudtrail/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.12.0](service/cloudwatch/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.12.0](service/connect/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.12.0](service/databasemigrationservice/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.13.0](service/databrew/CHANGELOG.md#v1130-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.10.0](service/devopsguru/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/drs`: [v1.0.0](service/drs/CHANGELOG.md#v100-2021-11-19)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/dynamodbstreams`: [v1.8.0](service/dynamodbstreams/CHANGELOG.md#v180-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.23.0](service/ec2/CHANGELOG.md#v1230-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.14.0](service/eks/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.14.0](service/forecast/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.10.0](service/ivs/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.10.0](service/kafka/CHANGELOG.md#v1100-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.16.0](service/kendra/CHANGELOG.md#v1160-2021-11-19)
  * **Announcement**: Fix API modeling bug incorrectly generating `DocumentAttributeValue` type as a union instead of a structure. This update corrects this bug by correcting the `DocumentAttributeValue` type to be a `struct` instead of an `interface`. This change also removes the `DocumentAttributeValueMember` types. To migrate to this change your application using service/kendra will need to be updated to use struct members in `DocumentAttributeValue` instead of `DocumentAttributeValueMember` types.
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.11.0](service/kms/CHANGELOG.md#v1110-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.13.0](service/lambda/CHANGELOG.md#v1130-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.13.0](service/lexmodelsv2/CHANGELOG.md#v1130-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.9.0](service/lexruntimev2/CHANGELOG.md#v190-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.9.0](service/location/CHANGELOG.md#v190-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.15.0](service/mediaconvert/CHANGELOG.md#v1150-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.14.0](service/medialive/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.7.0](service/mgn/CHANGELOG.md#v170-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/migrationhubstrategy`: [v1.0.0](service/migrationhubstrategy/CHANGELOG.md#v100-2021-11-19)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.9.0](service/qldb/CHANGELOG.md#v190-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/qldbsession`: [v1.9.0](service/qldbsession/CHANGELOG.md#v190-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.15.0](service/redshift/CHANGELOG.md#v1150-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.12.0](service/sns/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.16.0](service/ssm/CHANGELOG.md#v1160-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.12.0](service/transfer/CHANGELOG.md#v1120-2021-11-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.14.0](service/wafv2/CHANGELOG.md#v1140-2021-11-19)
  * **Feature**: API client updated

# Release (2021-11-12)

## General Highlights
* **Feature**: Service clients now support custom endpoints that have an initial URI path defined.
* **Feature**: Waiters now have a `WaitForOutput` method, which can be used to retrieve the output of the successful wait operation. Thank you to [Andrew Haines](https://github.com/haines) for contributing this feature.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.9.0](service/backup/CHANGELOG.md#v190-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.10.0](service/batch/CHANGELOG.md#v1100-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmeetings`: [v1.0.0](service/chimesdkmeetings/CHANGELOG.md#v100-2021-11-12)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.12.0](service/computeoptimizer/CHANGELOG.md#v1120-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.11.0](service/connect/CHANGELOG.md#v1110-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.12.0](service/docdb/CHANGELOG.md#v1120-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.8.0](service/dynamodb/CHANGELOG.md#v180-2021-11-12)
  * **Documentation**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.22.0](service/ec2/CHANGELOG.md#v1220-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.12.0](service/ecs/CHANGELOG.md#v1120-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.9.0](service/gamelift/CHANGELOG.md#v190-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.9.0](service/greengrassv2/CHANGELOG.md#v190-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.10.0](service/health/CHANGELOG.md#v1100-2021-11-12)
  * **Documentation**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.9.0](service/identitystore/CHANGELOG.md#v190-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.12.0](service/iotwireless/CHANGELOG.md#v1120-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.11.0](service/neptune/CHANGELOG.md#v1110-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.12.0](service/rds/CHANGELOG.md#v1120-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/resiliencehub`: [v1.0.0](service/resiliencehub/CHANGELOG.md#v100-2021-11-12)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi`: [v1.8.0](service/resourcegroupstaggingapi/CHANGELOG.md#v180-2021-11-12)
  * **Documentation**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.14.0](service/s3control/CHANGELOG.md#v1140-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.19.0](service/sagemaker/CHANGELOG.md#v1190-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime`: [v1.10.0](service/sagemakerruntime/CHANGELOG.md#v1100-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.7.0](service/ssmincidents/CHANGELOG.md#v170-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.11.0](service/transcribe/CHANGELOG.md#v1110-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/translate`: [v1.7.0](service/translate/CHANGELOG.md#v170-2021-11-12)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.13.0](service/wafv2/CHANGELOG.md#v1130-2021-11-12)
  * **Feature**: Updated service to latest API model.

# Release (2021-11-06)

## General Highlights
* **Feature**: The SDK now supports configuration of FIPS and DualStack endpoints using environment variables, shared configuration, or programmatically.
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream`: [v1.0.0](aws/protocol/eventstream/CHANGELOG.md#v100-2021-11-06)
  * **Announcement**: Support has been added for AWS EventStream APIs for Kinesis, S3, and Transcribe Streaming. Support for the Lex Runtime V2 EventStream API will be added in a future release.
  * **Release**: Protocol support has been added for AWS event stream.
* `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2`: [v2.0.0](internal/endpoints/v2/CHANGELOG.md#v200-2021-11-06)
  * **Release**: Endpoint Variant Model Support
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.6.0](service/applicationinsights/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.8.0](service/appstream/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.11.0](service/auditmanager/CHANGELOG.md#v1110-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.14.0](service/autoscaling/CHANGELOG.md#v1140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.13.0](service/chime/CHANGELOG.md#v1130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkidentity`: [v1.4.0](service/chimesdkidentity/CHANGELOG.md#v140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.4.0](service/chimesdkmessaging/CHANGELOG.md#v140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.10.0](service/cloudfront/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/codecommit`: [v1.7.0](service/codecommit/CHANGELOG.md#v170-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.10.0](service/connect/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/connectcontactlens`: [v1.7.0](service/connectcontactlens/CHANGELOG.md#v170-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/connectparticipant`: [v1.6.0](service/connectparticipant/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.10.0](service/databasemigrationservice/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.8.0](service/datasync/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.11.0](service/docdb/CHANGELOG.md#v1110-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.9.0](service/ebs/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.21.0](service/ec2/CHANGELOG.md#v1210-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.9.0](service/ecr/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.11.0](service/ecs/CHANGELOG.md#v1110-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.12.0](service/eks/CHANGELOG.md#v1120-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.13.0](service/elasticache/CHANGELOG.md#v1130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.9.0](service/elasticsearchservice/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/emrcontainers`: [v1.8.0](service/emrcontainers/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/finspace`: [v1.4.0](service/finspace/CHANGELOG.md#v140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.12.0](service/fsx/CHANGELOG.md#v1120-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.8.0](service/gamelift/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.9.0](service/health/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.12.0](service/iam/CHANGELOG.md#v1120-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/internal/eventstreamtesting`: [v1.0.0](service/internal/eventstreamtesting/CHANGELOG.md#v100-2021-11-06)
  * **Release**: Protocol support has been added for AWS event stream.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.13.0](service/iotsitewise/CHANGELOG.md#v1130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.14.0](service/kendra/CHANGELOG.md#v1140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.8.0](service/kinesis/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Support has been added for the SubscribeToShard API.
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.9.0](service/kms/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.12.0](service/lightsail/CHANGELOG.md#v1120-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.13.0](service/macie2/CHANGELOG.md#v1130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.6.0](service/mgn/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.10.0](service/neptune/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/networkmanager`: [v1.6.0](service/networkmanager/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.6.0](service/nimble/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.3.0](service/opensearch/CHANGELOG.md#v130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.14.0](service/quicksight/CHANGELOG.md#v1140-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.11.0](service/rds/CHANGELOG.md#v1110-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.10.0](service/rekognition/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53resolver`: [v1.9.0](service/route53resolver/CHANGELOG.md#v190-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.18.0](service/s3/CHANGELOG.md#v1180-2021-11-06)
  * **Feature**: Support has been added for the SelectObjectContent API.
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.13.0](service/s3control/CHANGELOG.md#v1130-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.18.0](service/sagemaker/CHANGELOG.md#v1180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/servicediscovery`: [v1.11.0](service/servicediscovery/CHANGELOG.md#v1110-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.6.0](service/ssmincidents/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sso`: [v1.6.0](service/sso/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.8.0](service/storagegateway/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/support`: [v1.7.0](service/support/CHANGELOG.md#v170-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.8.0](service/textract/CHANGELOG.md#v180-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.10.0](service/transcribe/CHANGELOG.md#v1100-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transcribestreaming`: [v1.0.0](service/transcribestreaming/CHANGELOG.md#v100-2021-11-06)
  * **Release**: New AWS service client module
  * **Feature**: Support has been added for the StartStreamTranscription and StartMedicalStreamTranscription APIs.
* `github.com/aws/aws-sdk-go-v2/service/waf`: [v1.6.0](service/waf/CHANGELOG.md#v160-2021-11-06)
  * **Feature**: Updated service to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.2.0](service/wisdom/CHANGELOG.md#v120-2021-11-06)
  * **Feature**: Updated service to latest API model.

# Release (2021-10-21)

## General Highlights
* **Feature**: Updated  to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.10.0
  * **Feature**: Adds dynamic signing middleware that switches to unsigned payload when TLS is enabled.
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.8.0](service/appflow/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationautoscaling`: [v1.8.0](service/applicationautoscaling/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.13.0](service/autoscaling/CHANGELOG.md#v1130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.3.0](service/chimesdkmessaging/CHANGELOG.md#v130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.11.0](service/cloudformation/CHANGELOG.md#v1110-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudsearch`: [v1.7.0](service/cloudsearch/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.7.0](service/cloudtrail/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.9.0](service/cloudwatch/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchevents`: [v1.7.0](service/cloudwatchevents/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.8.0](service/cloudwatchlogs/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codedeploy`: [v1.7.0](service/codedeploy/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.10.0](service/configservice/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dataexchange`: [v1.7.0](service/dataexchange/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.9.0](service/directconnect/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.10.0](service/docdb/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.6.0](service/dynamodb/CHANGELOG.md#v160-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.20.0](service/ec2/CHANGELOG.md#v1200-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.8.0](service/ecr/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.10.0](service/ecs/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.9.0](service/efs/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.12.0](service/elasticache/CHANGELOG.md#v1120-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing`: [v1.7.0](service/elasticloadbalancing/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.10.0](service/elasticloadbalancingv2/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.10.0](service/emr/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.8.0](service/eventbridge/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glacier`: [v1.6.0](service/glacier/CHANGELOG.md#v160-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.13.0](service/glue/CHANGELOG.md#v1130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.8.0](service/ivs/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.13.0](service/kendra/CHANGELOG.md#v1130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.7.0](service/kinesis/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalyticsv2`: [v1.7.0](service/kinesisanalyticsv2/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.8.0](service/kms/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.10.0](service/lambda/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.13.0](service/mediaconvert/CHANGELOG.md#v1130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.9.0](service/mediapackage/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.10.0](service/mediapackagevod/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.9.0](service/mediatailor/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.9.0](service/neptune/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/panorama`: [v1.0.0](service/panorama/CHANGELOG.md#v100-2021-10-21)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.13.0](service/quicksight/CHANGELOG.md#v1130-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.10.0](service/rds/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.12.0](service/redshift/CHANGELOG.md#v1120-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.10.0](service/robomaker/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.17.0](service/s3/CHANGELOG.md#v1170-2021-10-21)
  * **Feature**: Updates S3 streaming operations - PutObject, UploadPart, WriteGetObjectResponse to use unsigned payload signing auth when TLS is enabled.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.17.0](service/sagemaker/CHANGELOG.md#v1170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.12.0](service/securityhub/CHANGELOG.md#v1120-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sfn`: [v1.6.0](service/sfn/CHANGELOG.md#v160-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.9.0](service/sns/CHANGELOG.md#v190-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.10.0](service/sqs/CHANGELOG.md#v1100-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.7.0](service/storagegateway/CHANGELOG.md#v170-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.8.0](service/sts/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/swf`: [v1.6.0](service/swf/CHANGELOG.md#v160-2021-10-21)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.8.0](service/workmail/CHANGELOG.md#v180-2021-10-21)
  * **Feature**: API client updated

# Release (2021-10-11)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/feature/ec2/imds`: [v1.6.0](feature/ec2/imds/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: Respect passed in Context Deadline/Timeout. Updates the IMDS Client operations to not override the passed in Context's Deadline or Timeout options. If an Client operation is called with a Context with a Deadline or Timeout, the client will no longer override it with the client's default timeout.
  * **Bug Fix**: Fix IMDS client's response handling and operation timeout race. Fixes #1253
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.5.0](service/amplifybackend/CHANGELOG.md#v150-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationautoscaling`: [v1.7.0](service/applicationautoscaling/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.3.0](service/apprunner/CHANGELOG.md#v130-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.6.0](service/backup/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.11.0](service/chime/CHANGELOG.md#v1110-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.11.0](service/codebuild/CHANGELOG.md#v1110-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.10.0](service/databrew/CHANGELOG.md#v1100-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.19.0](service/ec2/CHANGELOG.md#v1190-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.8.0](service/efs/CHANGELOG.md#v180-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.9.0](service/elasticloadbalancingv2/CHANGELOG.md#v190-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/firehose`: [v1.7.0](service/firehose/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.10.0](service/frauddetector/CHANGELOG.md#v1100-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.10.0](service/fsx/CHANGELOG.md#v1100-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.12.0](service/glue/CHANGELOG.md#v1120-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/grafana`: [v1.0.0](service/grafana/CHANGELOG.md#v100-2021-10-11)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotevents`: [v1.8.0](service/iotevents/CHANGELOG.md#v180-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.12.0](service/kendra/CHANGELOG.md#v1120-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.7.0](service/kms/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.9.0](service/lexmodelsv2/CHANGELOG.md#v190-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.6.0](service/lexruntimev2/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.6.0](service/location/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.12.0](service/mediaconvert/CHANGELOG.md#v1120-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.10.0](service/medialive/CHANGELOG.md#v1100-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.16.0](service/sagemaker/CHANGELOG.md#v1160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.7.0](service/secretsmanager/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.11.0](service/securityhub/CHANGELOG.md#v1110-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.12.0](service/ssm/CHANGELOG.md#v1120-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.6.0](service/ssooidc/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.7.0](service/synthetics/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.6.0](service/textract/CHANGELOG.md#v160-2021-10-11)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.7.0](service/workmail/CHANGELOG.md#v170-2021-10-11)
  * **Feature**: API client updated

# Release (2021-09-30)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/account`: [v1.0.0](service/account/CHANGELOG.md#v100-2021-09-30)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.6.0](service/amp/CHANGELOG.md#v160-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appintegrations`: [v1.7.0](service/appintegrations/CHANGELOG.md#v170-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudcontrol`: [v1.0.0](service/cloudcontrol/CHANGELOG.md#v100-2021-09-30)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudhsmv2`: [v1.5.0](service/cloudhsmv2/CHANGELOG.md#v150-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.8.0](service/connect/CHANGELOG.md#v180-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dataexchange`: [v1.6.0](service/dataexchange/CHANGELOG.md#v160-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.8.0](service/elasticloadbalancingv2/CHANGELOG.md#v180-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.11.0](service/imagebuilder/CHANGELOG.md#v1110-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.9.0](service/lambda/CHANGELOG.md#v190-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.11.0](service/macie2/CHANGELOG.md#v1110-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.7.0](service/networkfirewall/CHANGELOG.md#v170-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.8.0](service/pinpoint/CHANGELOG.md#v180-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sesv2`: [v1.6.0](service/sesv2/CHANGELOG.md#v160-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.8.0](service/transfer/CHANGELOG.md#v180-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/voiceid`: [v1.0.0](service/voiceid/CHANGELOG.md#v100-2021-09-30)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wisdom`: [v1.0.0](service/wisdom/CHANGELOG.md#v100-2021-09-30)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workmail`: [v1.6.0](service/workmail/CHANGELOG.md#v160-2021-09-30)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.7.0](service/workspaces/CHANGELOG.md#v170-2021-09-30)
  * **Feature**: API client updated

# Release (2021-09-24)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression`: [v1.2.4](feature/dynamodb/expression/CHANGELOG.md#v124-2021-09-24)
  * **Documentation**: Fixes typo in NameBuilder.NamesList example documentation to use the correct variable name.
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.6.0](service/appmesh/CHANGELOG.md#v160-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.7.0](service/appsync/CHANGELOG.md#v170-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.9.0](service/auditmanager/CHANGELOG.md#v190-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codecommit`: [v1.5.0](service/codecommit/CHANGELOG.md#v150-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.8.0](service/comprehend/CHANGELOG.md#v180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.8.0](service/databasemigrationservice/CHANGELOG.md#v180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.18.0](service/ec2/CHANGELOG.md#v1180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.7.0](service/ecr/CHANGELOG.md#v170-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.7.0](service/elasticsearchservice/CHANGELOG.md#v170-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.10.0](service/iam/CHANGELOG.md#v1100-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.6.0](service/identitystore/CHANGELOG.md#v160-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.10.0](service/imagebuilder/CHANGELOG.md#v1100-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.13.0](service/iot/CHANGELOG.md#v1130-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotevents`: [v1.7.0](service/iotevents/CHANGELOG.md#v170-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kafkaconnect`: [v1.1.0](service/kafkaconnect/CHANGELOG.md#v110-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.6.0](service/lakeformation/CHANGELOG.md#v160-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.8.0](service/lexmodelsv2/CHANGELOG.md#v180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.5.0](service/lexruntimev2/CHANGELOG.md#v150-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.8.0](service/licensemanager/CHANGELOG.md#v180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.11.0](service/mediaconvert/CHANGELOG.md#v1110-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.9.0](service/mediapackagevod/CHANGELOG.md#v190-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.8.0](service/mediatailor/CHANGELOG.md#v180-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.1.0](service/opensearch/CHANGELOG.md#v110-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.12.0](service/quicksight/CHANGELOG.md#v1120-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.11.0](service/ssm/CHANGELOG.md#v1110-2021-09-24)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.10.0](service/wafv2/CHANGELOG.md#v1100-2021-09-24)
  * **Feature**: API client updated

# Release (2021-09-17)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.10.0](service/chime/CHANGELOG.md#v1100-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.10.1](service/cloudformation/CHANGELOG.md#v1101-2021-09-17)
  * **Documentation**: Updated API client documentation.
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.7.0](service/comprehend/CHANGELOG.md#v170-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.17.0](service/ec2/CHANGELOG.md#v1170-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ecr`: [v1.6.0](service/ecr/CHANGELOG.md#v160-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.12.0](service/iot/CHANGELOG.md#v1120-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/kafkaconnect`: [v1.0.0](service/kafkaconnect/CHANGELOG.md#v100-2021-09-17)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.7.0](service/lexmodelsv2/CHANGELOG.md#v170-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.4.0](service/lexruntimev2/CHANGELOG.md#v140-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.10.0](service/macie2/CHANGELOG.md#v1100-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.8.0](service/mediapackagevod/CHANGELOG.md#v180-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.6.0](service/networkfirewall/CHANGELOG.md#v160-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/pinpoint`: [v1.7.0](service/pinpoint/CHANGELOG.md#v170-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.11.0](service/quicksight/CHANGELOG.md#v1110-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.9.0](service/rds/CHANGELOG.md#v190-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.9.0](service/robomaker/CHANGELOG.md#v190-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.16.0](service/s3/CHANGELOG.md#v1160-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.15.0](service/sagemaker/CHANGELOG.md#v1150-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.5.0](service/ssooidc/CHANGELOG.md#v150-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.8.0](service/transcribe/CHANGELOG.md#v180-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.9.0](service/wafv2/CHANGELOG.md#v190-2021-09-17)
  * **Feature**: Updated API client and endpoints to latest revision.

# Release (2021-09-10)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.4.1](credentials/CHANGELOG.md#v141-2021-09-10)
  * **Documentation**: Fixes the AssumeRoleProvider's documentation for using custom TokenProviders.
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.5.0](service/amp/CHANGELOG.md#v150-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.7.0](service/braket/CHANGELOG.md#v170-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkidentity`: [v1.2.0](service/chimesdkidentity/CHANGELOG.md#v120-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.2.0](service/chimesdkmessaging/CHANGELOG.md#v120-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codegurureviewer`: [v1.7.0](service/codegurureviewer/CHANGELOG.md#v170-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.10.0](service/eks/CHANGELOG.md#v1100-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.11.0](service/elasticache/CHANGELOG.md#v1110-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.9.0](service/emr/CHANGELOG.md#v190-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.10.0](service/forecast/CHANGELOG.md#v1100-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.9.0](service/frauddetector/CHANGELOG.md#v190-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kafka`: [v1.7.0](service/kafka/CHANGELOG.md#v170-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.4.0](service/lookoutequipment/CHANGELOG.md#v140-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.8.0](service/mediapackage/CHANGELOG.md#v180-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opensearch`: [v1.0.0](service/opensearch/CHANGELOG.md#v100-2021-09-10)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.8.0](service/outposts/CHANGELOG.md#v180-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.7.0](service/ram/CHANGELOG.md#v170-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.14.0](service/sagemaker/CHANGELOG.md#v1140-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicediscovery`: [v1.9.0](service/servicediscovery/CHANGELOG.md#v190-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.5.0](service/ssmcontacts/CHANGELOG.md#v150-2021-09-10)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/xray`: [v1.6.0](service/xray/CHANGELOG.md#v160-2021-09-10)
  * **Feature**: API client updated

# Release (2021-09-02)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.8.0](config/CHANGELOG.md#v180-2021-09-02)
  * **Feature**: Add support for S3 Multi-Region Access Point ARNs.
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.7.0](service/accessanalyzer/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.8.0](service/acmpca/CHANGELOG.md#v180-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloud9`: [v1.8.0](service/cloud9/CHANGELOG.md#v180-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.10.0](service/cloudformation/CHANGELOG.md#v1100-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.6.0](service/cloudtrail/CHANGELOG.md#v160-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.10.0](service/codebuild/CHANGELOG.md#v1100-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.9.0](service/computeoptimizer/CHANGELOG.md#v190-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.9.0](service/configservice/CHANGELOG.md#v190-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.7.0](service/ebs/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.16.0](service/ec2/CHANGELOG.md#v1160-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.7.0](service/efs/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.8.0](service/emr/CHANGELOG.md#v180-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/firehose`: [v1.6.0](service/firehose/CHANGELOG.md#v160-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.8.0](service/frauddetector/CHANGELOG.md#v180-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.9.0](service/fsx/CHANGELOG.md#v190-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/internal/s3shared`: [v1.7.0](service/internal/s3shared/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: Add support for S3 Multi-Region Access Point ARNs.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.11.0](service/iot/CHANGELOG.md#v1110-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotjobsdataplane`: [v1.5.0](service/iotjobsdataplane/CHANGELOG.md#v150-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.7.0](service/ivs/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.6.0](service/kms/CHANGELOG.md#v160-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelbuildingservice`: [v1.9.0](service/lexmodelbuildingservice/CHANGELOG.md#v190-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.7.0](service/mediatailor/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/memorydb`: [v1.2.0](service/memorydb/CHANGELOG.md#v120-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mwaa`: [v1.5.0](service/mwaa/CHANGELOG.md#v150-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.6.0](service/polly/CHANGELOG.md#v160-2021-09-02)
  * **Feature**: API client updated
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.10.0](service/quicksight/CHANGELOG.md#v1100-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.15.0](service/s3/CHANGELOG.md#v1150-2021-09-02)
  * **Feature**: API client updated
  * **Feature**: Add support for S3 Multi-Region Access Point ARNs.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.11.0](service/s3control/CHANGELOG.md#v1110-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime`: [v1.7.0](service/sagemakerruntime/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/schemas`: [v1.6.0](service/schemas/CHANGELOG.md#v160-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.10.0](service/securityhub/CHANGELOG.md#v1100-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicecatalogappregistry`: [v1.5.0](service/servicecatalogappregistry/CHANGELOG.md#v150-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.9.0](service/sqs/CHANGELOG.md#v190-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.4.0](service/ssmincidents/CHANGELOG.md#v140-2021-09-02)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.7.0](service/transfer/CHANGELOG.md#v170-2021-09-02)
  * **Feature**: API client updated

# Release (2021-08-27)

## General Highlights
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.4.0](credentials/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Adds support for Tags and TransitiveTagKeys to stscreds.AssumeRoleProvider. Closes https://github.com/aws/aws-sdk-go-v2/issues/723
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`: [v1.2.0](feature/dynamodb/attributevalue/CHANGELOG.md#v120-2021-08-27)
  * **Bug Fix**: Fix unmarshaler's decoding of AttributeValueMemberN into a type that is a string alias.
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.7.0](service/acmpca/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.5.0](service/amplify/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.4.0](service/amplifybackend/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.7.0](service/apigateway/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi`: [v1.4.0](service/apigatewaymanagementapi/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.7.0](service/appflow/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/applicationinsights`: [v1.4.0](service/applicationinsights/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.2.0](service/apprunner/CHANGELOG.md#v120-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/appstream`: [v1.6.0](service/appstream/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.6.0](service/appsync/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.6.0](service/athena/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.8.0](service/auditmanager/CHANGELOG.md#v180-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/autoscalingplans`: [v1.5.0](service/autoscalingplans/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/backup`: [v1.5.0](service/backup/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.7.0](service/batch/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.6.0](service/braket/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkidentity`: [v1.1.0](service/chimesdkidentity/CHANGELOG.md#v110-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.1.0](service/chimesdkmessaging/CHANGELOG.md#v110-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.5.0](service/cloudtrail/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchevents`: [v1.6.0](service/cloudwatchevents/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/codeartifact`: [v1.5.0](service/codeartifact/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.9.0](service/codebuild/CHANGELOG.md#v190-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/codecommit`: [v1.4.0](service/codecommit/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/codeguruprofiler`: [v1.5.0](service/codeguruprofiler/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/codestarnotifications`: [v1.4.0](service/codestarnotifications/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentity`: [v1.5.0](service/cognitoidentity/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.6.0](service/cognitoidentityprovider/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/comprehend`: [v1.6.0](service/comprehend/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.8.0](service/computeoptimizer/CHANGELOG.md#v180-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/connectcontactlens`: [v1.5.0](service/connectcontactlens/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.9.0](service/customerprofiles/CHANGELOG.md#v190-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.7.0](service/databasemigrationservice/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.6.0](service/datasync/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/dax`: [v1.4.0](service/dax/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/directoryservice`: [v1.5.0](service/directoryservice/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/dlm`: [v1.5.0](service/dlm/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/dynamodbstreams`: [v1.4.0](service/dynamodbstreams/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.15.0](service/ec2/CHANGELOG.md#v1150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ecrpublic`: [v1.5.0](service/ecrpublic/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.6.0](service/efs/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.9.0](service/eks/CHANGELOG.md#v190-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/emrcontainers`: [v1.6.0](service/emrcontainers/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.7.0](service/eventbridge/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/finspace`: [v1.2.0](service/finspace/CHANGELOG.md#v120-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.2.0](service/finspacedata/CHANGELOG.md#v120-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/firehose`: [v1.5.0](service/firehose/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.7.0](service/fms/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.9.0](service/forecast/CHANGELOG.md#v190-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/forecastquery`: [v1.4.0](service/forecastquery/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.7.0](service/frauddetector/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.8.0](service/fsx/CHANGELOG.md#v180-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/gamelift`: [v1.6.0](service/gamelift/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.11.0](service/glue/CHANGELOG.md#v1110-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/groundstation`: [v1.6.0](service/groundstation/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/guardduty`: [v1.5.0](service/guardduty/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.7.0](service/health/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/healthlake`: [v1.6.0](service/healthlake/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.10.0](service/iot/CHANGELOG.md#v1100-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iot1clickdevicesservice`: [v1.4.0](service/iot1clickdevicesservice/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iotanalytics`: [v1.5.0](service/iotanalytics/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iotdataplane`: [v1.4.0](service/iotdataplane/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iotfleethub`: [v1.5.0](service/iotfleethub/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.11.0](service/iotsitewise/CHANGELOG.md#v1110-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ivs`: [v1.6.0](service/ivs/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.5.0](service/lakeformation/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.6.0](service/lexmodelsv2/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.3.0](service/lexruntimev2/CHANGELOG.md#v130-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.7.0](service/licensemanager/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.10.0](service/lightsail/CHANGELOG.md#v1100-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lookoutequipment`: [v1.3.0](service/lookoutequipment/CHANGELOG.md#v130-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.3.0](service/lookoutmetrics/CHANGELOG.md#v130-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.9.0](service/macie2/CHANGELOG.md#v190-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.10.0](service/mediaconvert/CHANGELOG.md#v1100-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mediapackage`: [v1.7.0](service/mediapackage/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.7.0](service/mediapackagevod/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mq`: [v1.5.0](service/mq/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/networkfirewall`: [v1.5.0](service/networkfirewall/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.7.0](service/outposts/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.6.0](service/pi/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoice`: [v1.4.0](service/pinpointsmsvoice/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.5.0](service/polly/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.6.0](service/qldb/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/qldbsession`: [v1.5.0](service/qldbsession/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.6.0](service/ram/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.8.0](service/rekognition/CHANGELOG.md#v180-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi`: [v1.5.0](service/resourcegroupstaggingapi/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.8.0](service/robomaker/CHANGELOG.md#v180-2021-08-27)
  * **Bug Fix**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycontrolconfig`: [v1.1.0](service/route53recoverycontrolconfig/CHANGELOG.md#v110-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/route53resolver`: [v1.7.0](service/route53resolver/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.14.0](service/s3/CHANGELOG.md#v1140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.10.0](service/s3control/CHANGELOG.md#v1100-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/s3outposts`: [v1.5.0](service/s3outposts/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/servicecatalog`: [v1.5.0](service/servicecatalog/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/servicecatalogappregistry`: [v1.4.0](service/servicecatalogappregistry/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/signer`: [v1.5.0](service/signer/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/ssooidc`: [v1.4.0](service/ssooidc/CHANGELOG.md#v140-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.6.0](service/storagegateway/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.6.0](service/synthetics/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.5.0](service/textract/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.7.0](service/transcribe/CHANGELOG.md#v170-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.6.0](service/transfer/CHANGELOG.md#v160-2021-08-27)
  * **Feature**: Updated API model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/wafregional`: [v1.5.0](service/wafregional/CHANGELOG.md#v150-2021-08-27)
  * **Feature**: Updated API model to latest revision.

# Release (2021-08-19)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/apigateway`: [v1.6.0](service/apigateway/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apigatewayv2`: [v1.5.0](service/apigatewayv2/CHANGELOG.md#v150-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.6.0](service/appflow/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/applicationautoscaling`: [v1.5.0](service/applicationautoscaling/CHANGELOG.md#v150-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloud9`: [v1.6.0](service/cloud9/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/clouddirectory`: [v1.4.0](service/clouddirectory/CHANGELOG.md#v140-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.6.0](service/cloudwatchlogs/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.8.0](service/codebuild/CHANGELOG.md#v180-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.7.0](service/configservice/CHANGELOG.md#v170-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.8.0](service/costexplorer/CHANGELOG.md#v180-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/customerprofiles`: [v1.8.0](service/customerprofiles/CHANGELOG.md#v180-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.8.0](service/databrew/CHANGELOG.md#v180-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/directoryservice`: [v1.4.0](service/directoryservice/CHANGELOG.md#v140-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.14.0](service/ec2/CHANGELOG.md#v1140-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.9.0](service/elasticache/CHANGELOG.md#v190-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.6.0](service/emr/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.10.0](service/iotsitewise/CHANGELOG.md#v1100-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.7.0](service/lambda/CHANGELOG.md#v170-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.6.0](service/licensemanager/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/memorydb`: [v1.0.0](service/memorydb/CHANGELOG.md#v100-2021-08-19)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.8.0](service/quicksight/CHANGELOG.md#v180-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.10.0](service/route53/CHANGELOG.md#v1100-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53resolver`: [v1.6.0](service/route53resolver/CHANGELOG.md#v160-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.13.0](service/s3/CHANGELOG.md#v1130-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.12.0](service/sagemaker/CHANGELOG.md#v1120-2021-08-19)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerruntime`: [v1.5.0](service/sagemakerruntime/CHANGELOG.md#v150-2021-08-19)
  * **Feature**: API client updated

# Release (2021-08-12)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign`: [v1.3.1](feature/cloudfront/sign/CHANGELOG.md#v131-2021-08-12)
  * **Bug Fix**: Update to not escape HTML when encoding the policy.
* `github.com/aws/aws-sdk-go-v2/service/athena`: [v1.5.0](service/athena/CHANGELOG.md#v150-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.11.0](service/autoscaling/CHANGELOG.md#v1110-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.8.0](service/chime/CHANGELOG.md#v180-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkidentity`: [v1.0.0](service/chimesdkidentity/CHANGELOG.md#v100-2021-08-12)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chimesdkmessaging`: [v1.0.0](service/chimesdkmessaging/CHANGELOG.md#v100-2021-08-12)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.7.0](service/codebuild/CHANGELOG.md#v170-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.6.0](service/connect/CHANGELOG.md#v160-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ebs`: [v1.5.0](service/ebs/CHANGELOG.md#v150-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.8.0](service/ecs/CHANGELOG.md#v180-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.5.0](service/lexmodelsv2/CHANGELOG.md#v150-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.9.0](service/lightsail/CHANGELOG.md#v190-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/nimble`: [v1.3.0](service/nimble/CHANGELOG.md#v130-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.7.0](service/rekognition/CHANGELOG.md#v170-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.9.0](service/route53/CHANGELOG.md#v190-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowdevicemanagement`: [v1.0.0](service/snowdevicemanagement/CHANGELOG.md#v100-2021-08-12)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.9.0](service/ssm/CHANGELOG.md#v190-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.5.0](service/synthetics/CHANGELOG.md#v150-2021-08-12)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.7.0](service/wafv2/CHANGELOG.md#v170-2021-08-12)
  * **Feature**: API client updated

# Release (2021-08-04)

## General Highlights
* **Feature**: adds error handling for defered close calls
* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.8.0
  * **Bug Fix**: Corrected an issue where the retryer was not using the last attempt's ResultMetadata as the bases for the return result from the stack. ([#1345](https://github.com/aws/aws-sdk-go-v2/pull/1345))
* `github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression`: [v1.2.0](feature/dynamodb/expression/CHANGELOG.md#v120-2021-08-04)
  * **Feature**: Add IsSet helper for ConditionBuilder and KeyConditionBuilder ([#1329](https://github.com/aws/aws-sdk-go-v2/pull/1329))
* `github.com/aws/aws-sdk-go-v2/service/accessanalyzer`: [v1.5.2](service/accessanalyzer/CHANGELOG.md#v152-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.3.1](service/amp/CHANGELOG.md#v131-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/appintegrations`: [v1.5.0](service/appintegrations/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.4.2](service/appmesh/CHANGELOG.md#v142-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/appsync`: [v1.5.0](service/appsync/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/auditmanager`: [v1.7.0](service/auditmanager/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/batch`: [v1.6.0](service/batch/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.5.2](service/braket/CHANGELOG.md#v152-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.7.0](service/chime/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.8.0](service/cloudformation/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.7.0](service/cloudwatch/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.6.0](service/codebuild/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/codeguruprofiler`: [v1.4.2](service/codeguruprofiler/CHANGELOG.md#v142-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.5.0](service/cognitoidentityprovider/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.7.0](service/computeoptimizer/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.7.0](service/databrew/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.7.0](service/directconnect/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.13.0](service/ec2/CHANGELOG.md#v1130-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.7.0](service/ecs/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.6.0](service/elasticloadbalancingv2/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/emr`: [v1.5.0](service/emr/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/emrcontainers`: [v1.5.0](service/emrcontainers/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.6.0](service/eventbridge/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.10.0](service/glue/CHANGELOG.md#v1100-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.5.0](service/greengrassv2/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/groundstation`: [v1.5.2](service/groundstation/CHANGELOG.md#v152-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.8.0](service/iam/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/identitystore`: [v1.4.0](service/identitystore/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.8.0](service/imagebuilder/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.9.0](service/iot/CHANGELOG.md#v190-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotanalytics`: [v1.4.0](service/iotanalytics/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.9.0](service/iotsitewise/CHANGELOG.md#v190-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.8.0](service/iotwireless/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.10.0](service/kendra/CHANGELOG.md#v1100-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.6.0](service/lambda/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lexmodelbuildingservice`: [v1.7.0](service/lexmodelbuildingservice/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.4.0](service/lexmodelsv2/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.4.0](service/location/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.9.0](service/mediaconvert/CHANGELOG.md#v190-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.8.0](service/medialive/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.3.1](service/mgn/CHANGELOG.md#v131-2021-08-04)
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.7.0](service/personalize/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.2.0](service/proton/CHANGELOG.md#v120-2021-08-04)
  * **Feature**: Updated to latest API model.
  * **Bug Fix**: Fixed an issue that caused one or more API operations to fail when attempting to resolve the service endpoint. ([#1349](https://github.com/aws/aws-sdk-go-v2/pull/1349))
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.5.0](service/qldb/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.7.0](service/quicksight/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.7.0](service/rds/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.10.0](service/redshift/CHANGELOG.md#v1100-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/redshiftdata`: [v1.5.0](service/redshiftdata/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/robomaker`: [v1.7.0](service/robomaker/CHANGELOG.md#v170-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.8.0](service/route53/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycluster`: [v1.0.0](service/route53recoverycluster/CHANGELOG.md#v100-2021-08-04)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53recoverycontrolconfig`: [v1.0.0](service/route53recoverycontrolconfig/CHANGELOG.md#v100-2021-08-04)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53recoveryreadiness`: [v1.0.0](service/route53recoveryreadiness/CHANGELOG.md#v100-2021-08-04)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.12.0](service/s3/CHANGELOG.md#v1120-2021-08-04)
  * **Feature**: Add `HeadObject` presign support. ([#1346](https://github.com/aws/aws-sdk-go-v2/pull/1346))
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.9.0](service/s3control/CHANGELOG.md#v190-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3outposts`: [v1.4.0](service/s3outposts/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.11.0](service/sagemaker/CHANGELOG.md#v1110-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/secretsmanager`: [v1.5.0](service/secretsmanager/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.8.0](service/securityhub/CHANGELOG.md#v180-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/shield`: [v1.6.0](service/shield/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.3.0](service/ssmcontacts/CHANGELOG.md#v130-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.2.0](service/ssmincidents/CHANGELOG.md#v120-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssoadmin`: [v1.5.0](service/ssoadmin/CHANGELOG.md#v150-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/synthetics`: [v1.4.0](service/synthetics/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/textract`: [v1.4.0](service/textract/CHANGELOG.md#v140-2021-08-04)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.6.0](service/transcribe/CHANGELOG.md#v160-2021-08-04)
  * **Feature**: Updated to latest API model.

# Release (2021-07-15)

## General Highlights
* **Dependency Update**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/config`: [v1.5.0](config/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Support has been added for EC2 IPv6-enabled Instance Metadata Service Endpoints.
* `github.com/aws/aws-sdk-go-v2/feature/ec2/imds`: [v1.3.0](feature/ec2/imds/CHANGELOG.md#v130-2021-07-15)
  * **Feature**: Support has been added for EC2 IPv6-enabled Instance Metadata Service Endpoints.
* `github.com/aws/aws-sdk-go-v2/service/acm`: [v1.5.0](service/acm/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.3.0](service/amp/CHANGELOG.md#v130-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.4.0](service/amplify/CHANGELOG.md#v140-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.3.0](service/amplifybackend/CHANGELOG.md#v130-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.10.0](service/autoscaling/CHANGELOG.md#v1100-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.6.0](service/chime/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.7.0](service/cloudformation/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.7.0](service/cloudfront/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/cloudsearch`: [v1.5.0](service/cloudsearch/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.6.0](service/cloudwatch/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/databasemigrationservice`: [v1.6.0](service/databasemigrationservice/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/devopsguru`: [v1.6.0](service/devopsguru/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/directconnect`: [v1.6.0](service/directconnect/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.8.0](service/docdb/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.12.0](service/ec2/CHANGELOG.md#v1120-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.8.0](service/eks/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.8.0](service/elasticache/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk`: [v1.5.0](service/elasticbeanstalk/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing`: [v1.5.0](service/elasticloadbalancing/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.5.0](service/elasticloadbalancingv2/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/fms`: [v1.6.0](service/fms/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/frauddetector`: [v1.6.0](service/frauddetector/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.9.0](service/glue/CHANGELOG.md#v190-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/health`: [v1.6.0](service/health/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/healthlake`: [v1.5.0](service/healthlake/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.7.0](service/iam/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.7.0](service/imagebuilder/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.8.0](service/iot/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.8.0](service/iotsitewise/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.9.0](service/kendra/CHANGELOG.md#v190-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/lambda`: [v1.5.0](service/lambda/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/lexmodelbuildingservice`: [v1.6.0](service/lexmodelbuildingservice/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.8.0](service/lightsail/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/macie`: [v1.5.1](service/macie/CHANGELOG.md#v151-2021-07-15)
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.8.1](service/macie2/CHANGELOG.md#v181-2021-07-15)
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.8.0](service/mediaconvert/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.5.0](service/mediatailor/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/mgn`: [v1.3.0](service/mgn/CHANGELOG.md#v130-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/mq`: [v1.4.0](service/mq/CHANGELOG.md#v140-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.7.0](service/neptune/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.6.0](service/outposts/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/pricing`: [v1.5.1](service/pricing/CHANGELOG.md#v151-2021-07-15)
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.6.0](service/rds/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.9.0](service/redshift/CHANGELOG.md#v190-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.10.0](service/sagemaker/CHANGELOG.md#v1100-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/ses`: [v1.5.0](service/ses/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.7.0](service/sns/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.7.0](service/sqs/CHANGELOG.md#v170-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.8.0](service/ssm/CHANGELOG.md#v180-2021-07-15)
  * **Feature**: Updated service model to latest version.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/storagegateway`: [v1.5.0](service/storagegateway/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.6.0](service/sts/CHANGELOG.md#v160-2021-07-15)
  * **Feature**: The ErrorCode method on generated service error types has been corrected to match the API model.
  * **Documentation**: Updated service model to latest revision.
* `github.com/aws/aws-sdk-go-v2/service/wellarchitected`: [v1.5.0](service/wellarchitected/CHANGELOG.md#v150-2021-07-15)
  * **Feature**: Updated service model to latest version.

# Release (2021-07-01)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/internal/ini`: [v1.1.0](internal/ini/CHANGELOG.md#v110-2021-07-01)
  * **Feature**: Support for `:`, `=`, `[`, `]` being present in expression values.
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.9.0](service/autoscaling/CHANGELOG.md#v190-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/databrew`: [v1.6.0](service/databrew/CHANGELOG.md#v160-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.11.0](service/ec2/CHANGELOG.md#v1110-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.8.0](service/glue/CHANGELOG.md#v180-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.8.0](service/kendra/CHANGELOG.md#v180-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.7.0](service/mediaconvert/CHANGELOG.md#v170-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediapackagevod`: [v1.6.0](service/mediapackagevod/CHANGELOG.md#v160-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.8.0](service/redshift/CHANGELOG.md#v180-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.9.0](service/sagemaker/CHANGELOG.md#v190-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/servicediscovery`: [v1.7.0](service/servicediscovery/CHANGELOG.md#v170-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.6.0](service/sqs/CHANGELOG.md#v160-2021-07-01)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.2.0](service/ssmcontacts/CHANGELOG.md#v120-2021-07-01)
  * **Feature**: API client updated

# Release (2021-06-25)

## General Highlights
* **Feature**: Updated `github.com/aws/smithy-go` to latest version
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.7.0
  * **Feature**: Adds configuration values for enabling endpoint discovery.
  * **Bug Fix**: Keep Object-Lock headers a header when presigning Sigv4 signing requests
* `github.com/aws/aws-sdk-go-v2/config`: [v1.4.0](config/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: Adds configuration setting for enabling endpoint discovery.
* `github.com/aws/aws-sdk-go-v2/credentials`: [v1.3.0](credentials/CHANGELOG.md#v130-2021-06-25)
  * **Bug Fix**: Fixed example usages of aws.CredentialsCache ([#1275](https://github.com/aws/aws-sdk-go-v2/pull/1275))
* `github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign`: [v1.2.0](feature/cloudfront/sign/CHANGELOG.md#v120-2021-06-25)
  * **Feature**: Add UnmarshalJSON for AWSEpochTime to correctly unmarshal AWSEpochTime, ([#1298](https://github.com/aws/aws-sdk-go-v2/pull/1298))
* `github.com/aws/aws-sdk-go-v2/internal/configsources`: [v1.0.0](internal/configsources/CHANGELOG.md#v100-2021-06-25)
  * **Release**: Release new modules
* `github.com/aws/aws-sdk-go-v2/service/amp`: [v1.2.0](service/amp/CHANGELOG.md#v120-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amplify`: [v1.3.0](service/amplify/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/amplifybackend`: [v1.2.0](service/amplifybackend/CHANGELOG.md#v120-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appflow`: [v1.5.0](service/appflow/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/appmesh`: [v1.4.0](service/appmesh/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/chime`: [v1.5.0](service/chime/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloud9`: [v1.5.0](service/cloud9/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudformation`: [v1.6.0](service/cloudformation/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.6.0](service/cloudfront/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudsearch`: [v1.4.0](service/cloudsearch/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatch`: [v1.5.0](service/cloudwatch/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchevents`: [v1.5.0](service/cloudwatchevents/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codebuild`: [v1.5.0](service/codebuild/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/codegurureviewer`: [v1.5.0](service/codegurureviewer/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentity`: [v1.4.0](service/cognitoidentity/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.4.0](service/cognitoidentityprovider/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.5.0](service/connect/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dax`: [v1.3.0](service/dax/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.7.0](service/docdb/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/dynamodb`: [v1.4.0](service/dynamodb/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: Adds support for endpoint discovery.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.10.0](service/ec2/CHANGELOG.md#v1100-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.7.0](service/elasticache/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk`: [v1.4.0](service/elasticbeanstalk/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing`: [v1.4.0](service/elasticloadbalancing/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2`: [v1.4.0](service/elasticloadbalancingv2/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eventbridge`: [v1.5.0](service/eventbridge/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/greengrass`: [v1.5.0](service/greengrass/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/greengrassv2`: [v1.4.0](service/greengrassv2/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.6.0](service/iam/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery`: [v1.0.0](service/internal/endpoint-discovery/CHANGELOG.md#v100-2021-06-25)
  * **Release**: Release new modules
  * **Feature**: Module supporting endpoint-discovery across all service clients.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.7.0](service/iot/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotanalytics`: [v1.3.0](service/iotanalytics/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.7.0](service/kendra/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kms`: [v1.4.0](service/kms/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.3.0](service/lexmodelsv2/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexruntimev2`: [v1.2.0](service/lexruntimev2/CHANGELOG.md#v120-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.5.0](service/licensemanager/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.2.0](service/lookoutmetrics/CHANGELOG.md#v120-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/managedblockchain`: [v1.4.0](service/managedblockchain/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconnect`: [v1.6.0](service/mediaconnect/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.7.0](service/medialive/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediatailor`: [v1.4.0](service/mediatailor/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.6.0](service/neptune/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.1.0](service/proton/CHANGELOG.md#v110-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.6.0](service/quicksight/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ram`: [v1.5.0](service/ram/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.5.0](service/rds/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshift`: [v1.7.0](service/redshift/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/redshiftdata`: [v1.4.0](service/redshiftdata/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.7.0](service/route53/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.8.0](service/sagemaker/CHANGELOG.md#v180-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakerfeaturestoreruntime`: [v1.4.0](service/sagemakerfeaturestoreruntime/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.7.0](service/securityhub/CHANGELOG.md#v170-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ses`: [v1.4.0](service/ses/CHANGELOG.md#v140-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/snowball`: [v1.5.0](service/snowball/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.6.0](service/sns/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.5.0](service/sqs/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sts`: [v1.5.0](service/sts/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/timestreamquery`: [v1.3.0](service/timestreamquery/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: Adds support for endpoint discovery.
* `github.com/aws/aws-sdk-go-v2/service/timestreamwrite`: [v1.3.0](service/timestreamwrite/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: Adds support for endpoint discovery.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.5.0](service/transfer/CHANGELOG.md#v150-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/waf`: [v1.3.0](service/waf/CHANGELOG.md#v130-2021-06-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/wafv2`: [v1.6.0](service/wafv2/CHANGELOG.md#v160-2021-06-25)
  * **Feature**: API client updated

# Release (2021-06-11)

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.7.0](service/autoscaling/CHANGELOG.md#v170-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudtrail`: [v1.3.2](service/cloudtrail/CHANGELOG.md#v132-2021-06-11)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider`: [v1.3.3](service/cognitoidentityprovider/CHANGELOG.md#v133-2021-06-11)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.6.0](service/eks/CHANGELOG.md#v160-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.6.0](service/fsx/CHANGELOG.md#v160-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/glue`: [v1.6.0](service/glue/CHANGELOG.md#v160-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.6.0](service/kendra/CHANGELOG.md#v160-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.7.0](service/macie2/CHANGELOG.md#v170-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/medialive`: [v1.6.0](service/medialive/CHANGELOG.md#v160-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/pi`: [v1.4.0](service/pi/CHANGELOG.md#v140-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/proton`: [v1.0.0](service/proton/CHANGELOG.md#v100-2021-06-11)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.3.1](service/qldb/CHANGELOG.md#v131-2021-06-11)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/rds`: [v1.4.2](service/rds/CHANGELOG.md#v142-2021-06-11)
  * **Documentation**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.7.0](service/sagemaker/CHANGELOG.md#v170-2021-06-11)
  * **Feature**: Updated to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.4.1](service/transfer/CHANGELOG.md#v141-2021-06-11)
  * **Documentation**: Updated to latest API model.

# Release (2021-06-04)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/acmpca`: [v1.5.0](service/acmpca/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.6.0](service/autoscaling/CHANGELOG.md#v160-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/braket`: [v1.4.0](service/braket/CHANGELOG.md#v140-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/cloudfront`: [v1.5.2](service/cloudfront/CHANGELOG.md#v152-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/datasync`: [v1.4.0](service/datasync/CHANGELOG.md#v140-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/devicefarm`: [v1.3.0](service/devicefarm/CHANGELOG.md#v130-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/docdb`: [v1.6.0](service/docdb/CHANGELOG.md#v160-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.9.0](service/ec2/CHANGELOG.md#v190-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.5.0](service/ecs/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.7.0](service/forecast/CHANGELOG.md#v170-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/fsx`: [v1.5.0](service/fsx/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.5.1](service/iam/CHANGELOG.md#v151-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/internal/s3shared`: [v1.4.0](service/internal/s3shared/CHANGELOG.md#v140-2021-06-04)
  * **Feature**: The handling of AccessPoint and Outpost ARNs have been updated.
* `github.com/aws/aws-sdk-go-v2/service/iotevents`: [v1.4.0](service/iotevents/CHANGELOG.md#v140-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ioteventsdata`: [v1.3.0](service/ioteventsdata/CHANGELOG.md#v130-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.6.0](service/iotsitewise/CHANGELOG.md#v160-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.6.0](service/iotwireless/CHANGELOG.md#v160-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/kendra`: [v1.5.0](service/kendra/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.6.1](service/lightsail/CHANGELOG.md#v161-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/location`: [v1.2.0](service/location/CHANGELOG.md#v120-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/mwaa`: [v1.2.0](service/mwaa/CHANGELOG.md#v120-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/outposts`: [v1.4.0](service/outposts/CHANGELOG.md#v140-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/polly`: [v1.3.0](service/polly/CHANGELOG.md#v130-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/qldb`: [v1.3.0](service/qldb/CHANGELOG.md#v130-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/resourcegroups`: [v1.3.2](service/resourcegroups/CHANGELOG.md#v132-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.6.2](service/route53/CHANGELOG.md#v162-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/route53resolver`: [v1.4.2](service/route53resolver/CHANGELOG.md#v142-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.10.0](service/s3/CHANGELOG.md#v1100-2021-06-04)
  * **Feature**: The handling of AccessPoint and Outpost ARNs have been updated.
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.7.0](service/s3control/CHANGELOG.md#v170-2021-06-04)
  * **Feature**: The handling of AccessPoint and Outpost ARNs have been updated.
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/servicediscovery`: [v1.5.0](service/servicediscovery/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sns`: [v1.5.0](service/sns/CHANGELOG.md#v150-2021-06-04)
  * **Feature**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/sqs`: [v1.4.2](service/sqs/CHANGELOG.md#v142-2021-06-04)
  * **Documentation**: Updated service client to latest API model.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.6.2](service/ssm/CHANGELOG.md#v162-2021-06-04)
  * **Documentation**: Updated service client to latest API model.

# Release (2021-05-25)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs`: [v1.4.0](service/cloudwatchlogs/CHANGELOG.md#v140-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/commander`: [v1.1.0](service/commander/CHANGELOG.md#v110-2021-05-25)
  * **Feature**: Deprecated module. The API client was incorrectly named. Use AWS Systems Manager Incident Manager (ssmincidents) instead.
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.5.0](service/computeoptimizer/CHANGELOG.md#v150-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/costexplorer`: [v1.6.0](service/costexplorer/CHANGELOG.md#v160-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.8.0](service/ec2/CHANGELOG.md#v180-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/efs`: [v1.4.0](service/efs/CHANGELOG.md#v140-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/forecast`: [v1.6.0](service/forecast/CHANGELOG.md#v160-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.6.0](service/iot/CHANGELOG.md#v160-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/opsworkscm`: [v1.4.0](service/opsworkscm/CHANGELOG.md#v140-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.5.0](service/quicksight/CHANGELOG.md#v150-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.9.0](service/s3/CHANGELOG.md#v190-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/ssmincidents`: [v1.0.0](service/ssmincidents/CHANGELOG.md#v100-2021-05-25)
  * **Release**: New AWS service client module
* `github.com/aws/aws-sdk-go-v2/service/transfer`: [v1.4.0](service/transfer/CHANGELOG.md#v140-2021-05-25)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/workspaces`: [v1.4.0](service/workspaces/CHANGELOG.md#v140-2021-05-25)
  * **Feature**: API client updated

# Release (2021-05-20)

## General Highlights
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.6.0
  * **Feature**: `internal/ini`: This package has been migrated to a separate module at `github.com/aws/aws-sdk-go-v2/internal/ini`.
* `github.com/aws/aws-sdk-go-v2/config`: [v1.3.0](config/CHANGELOG.md#v130-2021-05-20)
  * **Feature**: SSO credentials can now be defined alongside other credential providers within the same configuration profile.
  * **Bug Fix**: Profile names were incorrectly normalized to lower-case, which could result in unexpected profile configurations.
* `github.com/aws/aws-sdk-go-v2/internal/ini`: [v1.0.0](internal/ini/CHANGELOG.md#v100-2021-05-20)
  * **Release**: The `github.com/aws/aws-sdk-go-v2/internal/ini` package is now a Go Module.
* `github.com/aws/aws-sdk-go-v2/service/applicationcostprofiler`: [v1.0.0](service/applicationcostprofiler/CHANGELOG.md#v100-2021-05-20)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/apprunner`: [v1.0.0](service/apprunner/CHANGELOG.md#v100-2021-05-20)
  * **Release**: New AWS service client module
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/autoscaling`: [v1.5.0](service/autoscaling/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/computeoptimizer`: [v1.4.0](service/computeoptimizer/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/detective`: [v1.6.0](service/detective/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.5.0](service/eks/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticache`: [v1.6.0](service/elasticache/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/elasticsearchservice`: [v1.4.0](service/elasticsearchservice/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iam`: [v1.5.0](service/iam/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/imagebuilder`: [v1.5.0](service/imagebuilder/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.5.0](service/iot/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotdeviceadvisor`: [v1.4.0](service/iotdeviceadvisor/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/iotsitewise`: [v1.5.0](service/iotsitewise/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.4.0](service/kinesis/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalytics`: [v1.3.0](service/kinesisanalytics/CHANGELOG.md#v130-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalyticsv2`: [v1.4.0](service/kinesisanalyticsv2/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lexmodelsv2`: [v1.2.0](service/lexmodelsv2/CHANGELOG.md#v120-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/licensemanager`: [v1.4.0](service/licensemanager/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/lightsail`: [v1.6.0](service/lightsail/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie`: [v1.4.0](service/macie/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/macie2`: [v1.6.0](service/macie2/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/mediaconnect`: [v1.5.0](service/mediaconnect/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/neptune`: [v1.5.0](service/neptune/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/personalize`: [v1.5.0](service/personalize/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/quicksight`: [v1.4.0](service/quicksight/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/rekognition`: [v1.5.0](service/rekognition/CHANGELOG.md#v150-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.8.0](service/s3/CHANGELOG.md#v180-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemaker`: [v1.6.0](service/sagemaker/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/sagemakera2iruntime`: [v1.3.0](service/sagemakera2iruntime/CHANGELOG.md#v130-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/securityhub`: [v1.6.0](service/securityhub/CHANGELOG.md#v160-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/support`: [v1.3.0](service/support/CHANGELOG.md#v130-2021-05-20)
  * **Feature**: API client updated
* `github.com/aws/aws-sdk-go-v2/service/transcribe`: [v1.4.0](service/transcribe/CHANGELOG.md#v140-2021-05-20)
  * **Feature**: API client updated

# Release (2021-05-14)

## General Highlights
* **Feature**: Constant has been added to modules to enable runtime version inspection for reporting.
* **Dependency Update**: Updated to the latest SDK module versions

## Module Highlights
* `github.com/aws/aws-sdk-go-v2`: v1.5.0
  * **Feature**: `AddSDKAgentKey` and `AddSDKAgentKeyValue` in `aws/middleware` package have been updated to direct metadata to `User-Agent` HTTP header.
* `github.com/aws/aws-sdk-go-v2/service/codeartifact`: [v1.3.0](service/codeartifact/CHANGELOG.md#v130-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/commander`: [v1.0.0](service/commander/CHANGELOG.md#v100-2021-05-14)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/configservice`: [v1.5.0](service/configservice/CHANGELOG.md#v150-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/connect`: [v1.4.0](service/connect/CHANGELOG.md#v140-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/ec2`: [v1.7.0](service/ec2/CHANGELOG.md#v170-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/ecs`: [v1.4.0](service/ecs/CHANGELOG.md#v140-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/eks`: [v1.4.0](service/eks/CHANGELOG.md#v140-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/finspace`: [v1.0.0](service/finspace/CHANGELOG.md#v100-2021-05-14)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/finspacedata`: [v1.0.0](service/finspacedata/CHANGELOG.md#v100-2021-05-14)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/iot`: [v1.4.0](service/iot/CHANGELOG.md#v140-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/iotwireless`: [v1.5.0](service/iotwireless/CHANGELOG.md#v150-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/kinesis`: [v1.3.0](service/kinesis/CHANGELOG.md#v130-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalytics`: [v1.2.0](service/kinesisanalytics/CHANGELOG.md#v120-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/kinesisanalyticsv2`: [v1.3.0](service/kinesisanalyticsv2/CHANGELOG.md#v130-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/lakeformation`: [v1.3.0](service/lakeformation/CHANGELOG.md#v130-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/lookoutmetrics`: [v1.1.0](service/lookoutmetrics/CHANGELOG.md#v110-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/mediaconvert`: [v1.5.0](service/mediaconvert/CHANGELOG.md#v150-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/route53`: [v1.6.0](service/route53/CHANGELOG.md#v160-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/s3`: [v1.7.0](service/s3/CHANGELOG.md#v170-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/s3control`: [v1.6.0](service/s3control/CHANGELOG.md#v160-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/ssm`: [v1.6.0](service/ssm/CHANGELOG.md#v160-2021-05-14)
  * **Feature**: Updated to latest service API model.
* `github.com/aws/aws-sdk-go-v2/service/ssmcontacts`: [v1.0.0](service/ssmcontacts/CHANGELOG.md#v100-2021-05-14)
  * **Release**: New AWS service client module
  * **Feature**: Updated to latest service API model.

# Release 2021-05-06

## Breaking change
* `service/ec2` - v1.6.0
  * This release contains a breaking change to the Amazon EC2 API client. API number(int/int64/etc) and boolean members were changed from value, to pointer type. Your applications using the EC2 API client will fail to compile after upgrading for all members that were updated. To migrate to this module you'll need to update your application to use pointers for all number and boolean members in the API client module. The SDK provides helper utilities to convert between value and pointer types. For example the [aws.Bool](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#Bool) function to get the address from a bool literal. Similar utilities are available for all other primitive types in the [aws](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws) package.

## Service Client Highlights
* `service/acmpca` - v1.3.0
  * Feature: API client updated
* `service/apigateway` - v1.3.0
  * Feature: API client updated
* `service/auditmanager` - v1.4.0
  * Feature: API client updated
* `service/chime` - v1.3.0
  * Feature: API client updated
* `service/cloudformation` - v1.4.0
  * Feature: API client updated
* `service/cloudfront` - v1.4.0
  * Feature: API client updated
* `service/codegurureviewer` - v1.3.0
  * Feature: API client updated
* `service/connect` - v1.3.0
  * Feature: API client updated
* `service/customerprofiles` - v1.5.0
  * Feature: API client updated
* `service/devopsguru` - v1.3.0
  * Feature: API client updated
* `service/docdb` - v1.4.0
  * Feature: API client updated
* `service/ec2` - v1.6.0
  * Bug Fix: Fix incorrectly modeled Amazon EC2 number and boolean members in structures. The Amazon EC2 API client has been updated with a breaking change to fix all structure number and boolean members to be pointer types instead of value types. Fixes [#1107](https://github.com/aws/aws-sdk-go-v2/issues/1107), [#1178](https://github.com/aws/aws-sdk-go-v2/issues/1178), and [#1190](https://github.com/aws/aws-sdk-go-v2/issues/1190). This breaking change is made within the major version of the client' module, because the client operations failed and were unusable with value type number and boolean members with the EC2 API.
  * Feature: API client updated
* `service/ecs` - v1.3.0
  * Feature: API client updated
* `service/eks` - v1.3.0
  * Feature: API client updated
* `service/forecast` - v1.4.0
  * Feature: API client updated
* `service/glue` - v1.4.0
  * Feature: API client updated
* `service/health` - v1.3.0
  * Feature: API client updated
* `service/iotsitewise` - v1.3.0
  * Feature: API client updated
* `service/iotwireless` - v1.4.0
  * Feature: API client updated
* `service/kafka` - v1.3.0
  * Feature: API client updated
* `service/kinesisanalyticsv2` - v1.2.0
  * Feature: API client updated
* `service/macie2` - v1.4.0
  * Feature: API client updated
* `service/marketplacecatalog` - v1.2.0
  * Feature: API client updated
* `service/mediaconvert` - v1.4.0
  * Feature: API client updated
* `service/mediapackage` - v1.4.0
  * Feature: API client updated
* `service/mediapackagevod` - v1.3.0
  * Feature: API client updated
* `service/mturk` - v1.2.0
  * Feature: API client updated
* `service/nimble` - v1.0.0
  * Feature: API client updated
* `service/organizations` - v1.3.0
  * Feature: API client updated
* `service/personalize` - v1.3.0
  * Feature: API client updated
* `service/robomaker` - v1.4.0
  * Feature: API client updated
* `service/route53` - v1.5.0
  * Feature: API client updated
* `service/s3` - v1.6.0
  * Bug Fix: Fix PutObject and UploadPart unseekable stream documentation link to point to the correct location.
  * Feature: API client updated
* `service/sagemaker` - v1.4.0
  * Feature: API client updated
* `service/securityhub` - v1.4.0
  * Feature: API client updated
* `service/servicediscovery` - v1.3.0
  * Feature: API client updated
* `service/snowball` - v1.3.0
  * Feature: API client updated
* `service/sns` - v1.3.0
  * Feature: API client updated
* `service/ssm` - v1.5.0
  * Feature: API client updated
## Core SDK Highlights
* Dependency Update: Update smithy-go dependency to v1.4.0
* Dependency Update: Updated SDK dependencies to their latest versions.
* `aws` - v1.4.0
  * Feature: Add support for FIPS global partition endpoints ([#1242](https://github.com/aws/aws-sdk-go-v2/pull/1242))

# Release 2021-04-23
## Service Client Highlights
* `service/cloudformation` - v1.3.2
  * Documentation: Service Documentation Updates
* `service/cognitoidentityprovider` - v1.2.3
  * Documentation: Service Documentation Updates
* `service/costexplorer` - v1.4.0
  * Feature: Service API Updates
* `service/databasemigrationservice` - v1.3.0
  * Feature: Service API Updates
* `service/detective` - v1.4.0
  * Feature: Service API Updates
* `service/elasticache` - v1.4.0
  * Feature: Service API Updates
* `service/forecast` - v1.3.0
  * Feature: Service API Updates
* `service/groundstation` - v1.3.0
  * Feature: Service API Updates
* `service/kendra` - v1.3.0
  * Feature: Service API Updates
* `service/redshift` - v1.5.0
  * Feature: Service API Updates
* `service/savingsplans` - v1.2.0
  * Feature: Service API Updates
* `service/securityhub` - v1.3.0
  * Feature: Service API Updates
## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.
* `feature/rds/auth` - v1.0.0
  * Feature: Add Support for Amazon RDS IAM Authentication

# Release 2021-04-14
## Service Client Highlights
* `service/codebuild` - v1.3.0
  * Feature: API client updated
* `service/codestarconnections` - v1.2.0
  * Feature: API client updated
* `service/comprehendmedical` - v1.2.0
  * Feature: API client updated
* `service/configservice` - v1.4.0
  * Feature: API client updated
* `service/ec2` - v1.5.0
  * Feature: API client updated
* `service/fsx` - v1.3.0
  * Feature: API client updated
* `service/lightsail` - v1.4.0
  * Feature: API client updated
* `service/mediaconnect` - v1.3.0
  * Feature: API client updated
* `service/rds` - v1.3.0
  * Feature: API client updated
* `service/redshift` - v1.4.0
  * Feature: API client updated
* `service/shield` - v1.3.0
  * Feature: API client updated
* `service/sts` - v1.3.0
  * Feature: API client updated
## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.

# Release 2021-04-08
## Service Client Highlights
* Feature: API model sync
* `service/lookoutequipment` - v1.0.0
  * v1 Release: new service client
* `service/mgn` - v1.0.0
  * v1 Release: new service client
## Core SDK Highlights
* Dependency Update: smithy-go version bump
* Dependency Update: Updated SDK dependencies to their latest versions.

# Release 2021-04-01
## Service Client Highlights
* Bug Fix: Fix URL Path and RawQuery of resolved endpoint being ignored by the API client's request serialization.
  * Fixes [issue#1191](https://github.com/aws/aws-sdk-go-v2/issues/1191)
* Refactored internal endpoints model for accessors
* Feature: updated to latest models
* New services 
  * `service/location` - v1.0.0
  * `service/lookoutmetrics` - v1.0.0
## Core SDK Highlights
* Dependency Update: update smithy-go module
* Dependency Update: Updated SDK dependencies to their latest versions.

# Release 2021-03-18
## Service Client Highlights
* Bug Fix: Updated presign URLs to no longer include the X-Amz-User-Agent header
* Feature: Update API model
* Add New supported API
* `service/internal/s3shared` - v1.2.0
  * Feature: Support for S3 Object Lambda
* `service/s3` - v1.3.0
  * Bug Fix: Adds documentation to the PutObject and UploadPart operations Body member how to upload unseekable objects to an Amazon S3 Bucket.
  * Feature: S3 Object Lambda is a new S3 feature that enables users to apply their own custom code to process the output of a standard S3 GET request by automatically invoking a Lambda function with a GET request
* `service/s3control` - v1.3.0
  * Feature: S3 Object Lambda is a new S3 feature that enables users to apply their own custom code to process the output of a standard S3 GET request by automatically invoking a Lambda function with a GET request
## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.
* `aws` - v1.3.0
  * Feature: Add helper to V4 signer package to swap compute payload hash middleware with unsigned payload middleware
* `feature/s3/manager` - v1.1.0
  * Bug Fix: Add support for Amazon S3 Object Lambda feature.
  * Feature: Updates for S3 Object Lambda feature

# Release 2021-03-12
## Service Client Highlights
* Bug Fix: Fixed a bug that could union shape types to be deserialized incorrectly
* Bug Fix: Fixed a bug where unboxed shapes that were marked as required were not serialized and sent over the wire, causing an API error from the service.
* Bug Fix: Fixed a bug with generated API Paginators' handling of nil input parameters causing a panic.
* Dependency Update: update smithy-go dependency
* `service/detective` - v1.1.2
  * Bug Fix: Fix deserialization of API response timestamp member.
* `service/docdb` - v1.2.0
  * Feature: Client now support presigned URL generation for CopyDBClusterSnapshot and CreateDBCluster operations by specifying the target SourceRegion
* `service/neptune` - v1.2.0
  * Feature: Client now support presigned URL generation for CopyDBClusterSnapshot and CreateDBCluster operations by specifying the target SourceRegion
* `service/s3` - v1.2.1
  * Bug Fix: Fixed an issue where ListObjectsV2 and ListParts paginators could loop infinitely
  * Bug Fix: Fixed key encoding when addressing S3 Access Points
## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.
* `config` - v1.1.2
  * Bug Fix: Fixed a panic when using WithEC2IMDSRegion without a specified IMDS client

# Release 2021-02-09
## Service Client Highlights
* `service/s3` - v1.2.0
  * Feature: adds support for s3 vpc endpoint interface [#1113](https://github.com/aws/aws-sdk-go-v2/pull/1113)
* `service/s3control` - v1.2.0
  * Feature: adds support for s3 vpc endpoint interface [#1113](https://github.com/aws/aws-sdk-go-v2/pull/1113)
## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.
* `aws` - v1.2.0
  * Feature: support to add endpoint source on context. Adds getter/setter for the endpoint source [#1113](https://github.com/aws/aws-sdk-go-v2/pull/1113)
* `config` - v1.1.1
  * Bug Fix: Only Validate SSO profile configuration when attempting to use SSO credentials [#1103](https://github.com/aws/aws-sdk-go-v2/pull/1103)
  * Bug Fix: Environment credentials were not taking precedence over AWS_PROFILE [#1103](https://github.com/aws/aws-sdk-go-v2/pull/1103)

# Release 2021-01-29
## Service Client Highlights
* Bug Fix: A serialization bug has been fixed that caused some service operations with empty inputs to not be serialized correctly ([#1071](https://github.com/aws/aws-sdk-go-v2/pull/1071))
* Bug Fix: Fixes a bug that could cause a waiter to fail when comparing types ([#1083](https://github.com/aws/aws-sdk-go-v2/pull/1083))
## Core SDK Highlights
* Feature: EndpointResolverFromURL helpers have been added for constructing a service EndpointResolver type ([#1066](https://github.com/aws/aws-sdk-go-v2/pull/1066))
* Dependency Update: Updated SDK dependencies to their latest versions.
* `aws` - v1.1.0
  * Feature: Add support for specifying the EndpointSource on aws.Endpoint types ([#1070](https://github.com/aws/aws-sdk-go-v2/pull/1070/))
* `config` - v1.1.0
  * Feature: Add Support for AWS Single Sign-On (SSO) credential provider ([#1072](https://github.com/aws/aws-sdk-go-v2/pull/1072))
* `credentials` - v1.1.0
  * Feature: Add AWS Single Sign-On (SSO) credential provider ([#1072](https://github.com/aws/aws-sdk-go-v2/pull/1072))

# Release 2021-01-19

We are excited to announce the [General Availability](https://aws.amazon.com/blogs/developer/aws-sdk-for-go-version-2-general-availability/)
(GA) release of the [AWS SDK for Go version 2 (v2)](https://github.com/aws/aws-sdk-go-v2).
This release follows the [Release candidate](https://aws.amazon.com/blogs/developer/aws-sdk-for-go-version-2-v2-release-candidate)
of the AWS SDK for Go v2. Version 2 incorporates customer feedback from version 1 and takes advantage of modern Go language features.

## Breaking Changes
* `aws`: Updated Config.Retryer member to be a func that returns aws.Retryer ([#1033](https://github.com/aws/aws-sdk-go-v2/pull/1033))
    * Updates the SDK's references to Config.Retryer to be a function that returns aws.Retryer value. This ensures that custom retry options specified in the `aws.Config` are scoped to individual client instances. 
    * All API clients created with the config will call the `Config.Retryer` function to get an aws.Retryer.
    * Removes duplicate `Retryer` interface from `retry` package. Single definition is `aws.Retryer` now.
* `aws/middleware`: Updates `AddAttemptClockSkewMiddleware` to use appropriate `AddRecordResponseTiming` naming ([#1031](https://github.com/aws/aws-sdk-go-v2/pull/1031))
    * Removes `ResponseMetadata` struct type, and adds its members to middleware metadata directly, to improve discoverability.
* `config`: Updated the `WithRetryer` helper to take a function that returns an aws.Retryer ([#1033](https://github.com/aws/aws-sdk-go-v2/pull/1033))
    * All API clients created with the config will call the `Config.Retryer` function to get an aws.Retryer.
* `API Clients`: Fix SDK's API client enum constant name generation to have expected casing ([#1020](https://github.com/aws/aws-sdk-go-v2/pull/1020))
    * This updates of the generated enum const value names in API client's `types` package to have the expected casing. Prior to this, enum names were being generated with lowercase names instead of camel case. 
* `API Clients`: Updates SDK's API client request middleware stack values to be scoped to individual operation call ([#1019](https://github.com/aws/aws-sdk-go-v2/pull/1019))
    * The API client request middleware stack values were mistakenly allowed to escape to nested API operation calls. This broke the SDK's presigners.
    * Stack values that should not escape are not scoped to the individual operation call.
* `Multiple API Clients`: Unexported the API client's `WithEndpointResolver` this type wasn't intended to be exported ([#1051](https://github.com/aws/aws-sdk-go-v2/pull/1051))
    * Using the `aws.Config.EndpointResolver` member for setting custom endpoint resolver instead.

## New Features
* `service/sts`: Add support for presigning GetCallerIdentity operation ([#1030](https://github.com/aws/aws-sdk-go-v2/pull/1030))
    * Adds a PresignClient to the `sts` API client module. Use PresignGetCallerIdentity to obtain presigned URLs for the create presigned URLs for the GetCallerIdentity operation.
    * Fixes [#1021](https://github.com/aws/aws-sdk-go-v2/issues/1021)
* `aws/retry`: Add package documentation for retry package ([#1033](https://github.com/aws/aws-sdk-go-v2/pull/1033))
    * Adds documentation for the retry package 

## Bug Fixes
* `Multiple API Clients`: Fix SDK's generated serde for unmodeled operation input/output ([#1050](https://github.com/aws/aws-sdk-go-v2/pull/1050))
    * Fixes [#1047](https://github.com/aws/aws-sdk-go-v2/issues/1047) by fixing the how the SDKs generated serialization and deserialization of API operations that did not have modeled input or output types. This caused the SDK to incorrectly attempt to deserialize response documents that were either empty, or contained unexpected data.
* `service/s3`: Fix Tagging parameter not serialized correctly for presigned PutObject requests ([#1017](https://github.com/aws/aws-sdk-go-v2/pull/1017))
    * Fixes the Tagging parameter incorrectly being serialized to the URL's query string instead of being signed as a HTTP request header.
    * When using PresignPutObject make sure to add all signed headers returned by the method to your down stream's HTTP client's request. These headers must be included in the request, or the request will fail with signature errors.
    * Fixes [#1016](https://github.com/aws/aws-sdk-go-v2/issues/1016)
* `service/s3`: Fix Unmarshaling `GetObjectAcl` operation's Grantee type response ([#1034](https://github.com/aws/aws-sdk-go-v2/pull/1034))
    * Updates the SDK's codegen for correctly deserializing XML attributes in tags with XML namespaces.
    * Fixes [#1013](https://github.com/aws/aws-sdk-go-v2/issues/1013)
* `service/s3`: Fix Unmarshaling `GetBucketLocation` operation's response ([#1027](https://github.com/aws/aws-sdk-go-v2/pull/1027))
    * Fixes [#908](https://github.com/aws/aws-sdk-go-v2/issues/908)

## Migrating from v2 preview SDK's v0.31.0 to v1.0.0

### aws.Config Retryer member

If your application sets the `Config.Retryer` member the application will need
to be updated to set a function that returns an `aws.Retryer`. In addition, if
your application used the `config.WithRetryer` helper a function that returns
an `aws.Retryer` needs to be used.

If your application used the `retry.Retryer` type, update to using the
`aws.Retryer` type instead.

### API Client enum value names

If your application used the enum values in the API Client's `types` package between v0.31.0 and the latest version of the client module you may need to update the naming of the enum value. The enum value name casing were updated to camel case instead lowercased.

# Release 2020-12-23

We’re happy to announce the Release Candidate (RC) of the AWS SDK for Go v2.
This RC follows the developer preview release of the AWS SDK for Go v2. The SDK
has undergone a major rewrite from the v1 code base to incorporate your
feedback and to take advantage of modern Go language features.

## Documentation
* Developer Guide: https://aws.github.io/aws-sdk-go-v2/docs/
* API Reference docs: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2
* Migration Guide: https://aws.github.io/aws-sdk-go-v2/docs/migrating/

## Breaking Changes
* Dependency `github.com/awslabs/smithy-go` has been relocated to `github.com/aws/smithy-go`
    * The `smithy-go` repository was moved from the `awslabs` GitHub organization to `aws`.
    * `xml`, `httpbinding`, and `json` package relocated under `encoding` package.
* The module `ec2imds` moved to `feature/ec2/imds` path ([#984](https://github.com/aws/aws-sdk-go-v2/pull/984))
    * Moves the `ec2imds` feature module to be in common location as other SDK features.
* `aws/signer/v4`: Refactor AWS Sigv4 Signer and options types to allow function options ([#955](https://github.com/aws/aws-sdk-go-v2/pull/955))
    * Fixes [#917](https://github.com/aws/aws-sdk-go-v2/issues/917), [#960](https://github.com/aws/aws-sdk-go-v2/issues/960), [#958](https://github.com/aws/aws-sdk-go-v2/issues/958)
* `aws`: CredentialCache type updated to require constructor function ([#946](https://github.com/aws/aws-sdk-go-v2/pull/946))
    * Fixes [#940](https://github.com/aws/aws-sdk-go-v2/issues/940)
* `credentials`: ExpiryWindow and Jitter moved from credential provider to `CredentialCache` ([#946](https://github.com/aws/aws-sdk-go-v2/pull/946))
    * Moves ExpiryWindow and Jitter options to common option of the `CredentialCache` instead of duplicated across providers.
    * Fixes [#940](https://github.com/aws/aws-sdk-go-v2/issues/940)
* `config`: Ensure shared credentials file has precedence over shared config file ([#990](https://github.com/aws/aws-sdk-go-v2/pull/990))
    * The shared config file was incorrectly overriding the shared credentials file when merging values.
* `config`: Add `context.Context` to `LoadDefaultConfig` ([#951](https://github.com/aws/aws-sdk-go-v2/pull/951))
    * Updates `config#LoadDefaultConfig` function to take `context.Context` as well as functional options for the `config#LoadOptions` type.
    * Fixes [#926](https://github.com/aws/aws-sdk-go-v2/issues/926), [#819](https://github.com/aws/aws-sdk-go-v2/issues/819)
* `aws`: Rename `NoOpRetryer` to `NopRetryer` to have consistent naming with rest of SDK ([#987](https://github.com/aws/aws-sdk-go-v2/pull/987))
    * Fixes [#878](https://github.com/aws/aws-sdk-go-v2/issues/878)
* `service/s3control`: Change `S3InitiateRestoreObjectOperation.ExpirationInDays` from value to pointer type ([#988](https://github.com/aws/aws-sdk-go-v2/pull/988))
* `aws`: `ReaderSeekerCloser` and `WriteAtBuffer` have been relocated to `feature/s3/manager`.

## New Features
* *Waiters*: Add Waiter utilities for API clients ([aws/smithy-go#237](https://github.com/aws/smithy-go/pull/237))
    * Your application can now use Waiter utilities to wait for AWS resources.
* `feature/dynamodb/attributevalue`: Add Amazon DynamoDB Attribute value marshaler utility ([#948](https://github.com/aws/aws-sdk-go-v2/pull/948))
    * Adds a utility for marshaling Go types too and from Amazon DynamoDB AttributeValues.
    * Also includes utility for converting from Amazon DynamoDB Streams AttributeValues to Amazon DynamoDB AttributeValues.
* `feature/dynamodbstreams/attributevalue`: Add Amazon DynamoDB Streams Attribute value marshaler utility ([#948](https://github.com/aws/aws-sdk-go-v2/pull/948))
    * Adds a utility for marshaling Go types too and from Amazon DynamoDB Streams AttributeValues.
    * Also includes utility for converting from Amazon DynamoDB AttributeValues to Amazon DynamoDB Streams AttributeValues.
* `feature/dynamodb/expression`: Add Amazon DynamoDB expression utility ([#981](https://github.com/aws/aws-sdk-go-v2/pull/981))
    * Adds the expression utility to the SDK for easily building Amazon DynamoDB operation expressions in code.

## Bug Fixes
* `service/s3`: Fix Presigner to configure client correctly for Amazon S3 ([#969](https://github.com/aws/aws-sdk-go-v2/pull/969))
* service/s3: Fix deserialization of CompleteMultipartUpload ([#965](https://github.com/aws/aws-sdk-go-v2/pull/965)
    * Fixes [#927](https://github.com/aws/aws-sdk-go-v2/issues/927)
* `codegen`: Fix API client union serialization ([#979](https://github.com/aws/aws-sdk-go-v2/pull/979))
    * Fixes [#978](https://github.com/aws/aws-sdk-go-v2/issues/978)

## Service Client Highlights
* API Clients have been bumped to version `v0.31.0`
* Regenerate API Clients from updated API models adding waiter utilities, and union parameters.
* `codegen`:
    * Add documentation to union API parameters describing valid member types, and usage example ([aws/smithy-go#239](https://github.com/aws/smithy-go/pull/239))
    * Normalize Metadata header map keys to be lower case ([aws/smithy-go#241](https://github.com/aws/smithy-go/pull/241)), ([#982](https://github.com/aws/aws-sdk-go-v2/pull/982))
        * Fixes [#376](https://github.com/aws/aws-sdk-go-v2/issues/376) Amazon S3 Metadata parameters keys are always returned as lower case.
    * Fix API client deserialization of XML based responses ([aws/smithy-go#245](https://github.com/aws/smithy-go/pull/245)), ([#992](https://github.com/aws/aws-sdk-go-v2/pull/992))
        * Fixes [#910](https://github.com/aws/aws-sdk-go-v2/issues/910)
* `service/s3`, `service/s3control`:
    * Add support for reading `s3_use_arn_region` from shared config file ([#991](https://github.com/aws/aws-sdk-go-v2/pull/991))
    * Add Utility for getting RequestID and HostID of response ([#983](https://github.com/aws/aws-sdk-go-v2/pull/983))


## Other changes
* Updates branch `HEAD` points from `master` to `main`.
    * This should not impact your application, but if you have pull requests or forks of the SDK you may need to update the upstream branch your fork is based off of.

## Migrating from v2 preview SDK's v0.30.0 to v0.31.0 release candidate

### smithy-go module relocation

If your application uses `smithy-go` utilities for request pipeline your application will need to be updated to refer to the new import path of `github.com/aws/smithy-go`. If you application did *not* use `smithy-go` utilities directly, your application will update automatically.

### EC2 IMDS module relocation

If your application used the `ec2imds` module, it has been relocated to `feature/ec2/imds`. Your application will need to update to the new import path, `github.com/aws/aws-sdk-go-v2/feature/ec2/imds`.

### CredentialsCache Constructor and ExpiryWindow Options

The `aws#CredentialsCache` type was updated, and a new constructor function, `NewCredentialsCache` was added. This function needs to be used to initialize the `CredentialCache`. The constructor also has function options to specify additional configuration, e.g. ExpiryWindow and Jitter.

If your application was specifying the `ExpiryWindow` with the `credentials/stscreds#AssumeRoleOptions`, `credentials/stscreds#WebIdentityRoleOptions`, `credentials/processcreds#Options`, or `credentials/ec2rolecrds#Options` types the `ExpiryWindow` option will need to specified on the `CredentialsCache` constructor instead.

### AWS Sigv4 Signer Refactor

The `aws/signer/v4` package's `Signer.SignHTTP` and `Signer.PresignHTTP` methods were updated to take functional options. If your application provided a custom implementation for API client's `HTTPSignerV4` or `HTTPPresignerV4` interfaces, that implementation will need to be updated for the new function signature.

### Configuration Loading

The `config#LoadDefaultConfig` function has been updated to require a `context.Context` as the first parameter, with additional optional function options as variadic additional arguments. Your application will need to update its usage of `LoadDefaultConfig` to pass in `context.Context` as the first parameter. If your application used the `With...` helpers those should continue to work without issue.

The v2 SDK corrects its behavior to be inline with the AWS CLI and other AWS SDKs. Refer to https://docs.aws.amazon.com/credref/latest/refdocs/overview.html for more information how to use the shared config and credentials files.


# Release 2020-11-30

## Breaking Change
* `codegen`: Add support for slice and maps generated with value members instead of pointer ([#887](https://github.com/aws/aws-sdk-go-v2/pull/887))
    * This update allow the SDK's code generation to be aware of API shapes and members that are not nullable, and can be rendered as value types by the code generation instead of pointer types.
    * Several API client parameter types will change from pointer members to value members for slice, map, number and bool member types.
    * See Migration notes for migrating to v0.30.0 with this change.
* `aws/transport/http`: Move aws.BuildableHTTPClient to HTTP transport package ([#898](https://github.com/aws/aws-sdk-go-v2/pull/898))
    * Moves the `BuildableHTTPClient` from the SDK's `aws` package to the `aws/transport/http` package as `BuildableClient` to with other HTTP specific utilities.
* `feature/cloudfront/sign`: Add CloudFront sign feature as module ([#884](https://github.com/aws/aws-sdk-go-v2/pull/884))
    * Moves `service/cloudfront/sign` package out of the `cloudfront` module, and into its own module as `github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign`.

## New Features
* `config`: Add a WithRetryer provider helper to the config loader ([#897](https://github.com/aws/aws-sdk-go-v2/pull/897))
    * Adds a `WithRetryer` configuration provider to the config loader as a convenience helper to set the `Retryer` on the `aws.Config` when its being loaded.
* `config`: Default to TLS 1.2 for HTTPS requests ([#892](https://github.com/aws/aws-sdk-go-v2/pull/892))
    * Updates the SDK's default HTTP client to use TLS 1.2 as the minimum TLS version for all HTTPS requests by default.

## Bug Fixes
* `config`: Fix AWS_CA_BUNDLE usage while loading default config ([#912](https://github.com/aws/aws-sdk-go-v2/pull/))
    * Fixes the `LoadDefaultConfig`'s configuration provider order to correctly load a custom HTTP client prior to configuring the client for `AWS_CA_BUNDLE` environment variable.
* `service/s3`: Fix signature mismatch error for s3 ([#913](https://github.com/aws/aws-sdk-go-v2/pull/913))
    * Fixes ([#883](https://github.com/aws/aws-sdk-go-v2/issues/883))
* `service/s3control`:
    * Fix HostPrefix addition behavior for s3control ([#882](https://github.com/aws/aws-sdk-go-v2/pull/882))
        * Fixes ([#863](https://github.com/aws/aws-sdk-go-v2/issues/863))
    * Fix s3control error deserializer ([#875](https://github.com/aws/aws-sdk-go-v2/pull/875))
        * Fixes ([#864](https://github.com/aws/aws-sdk-go-v2/issues/864))

## Service Client Highlights
* Pagination support has been added to supported APIs. See [Using Operation Paginators](https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#using-operation-paginators) in the Developer Guide. ([#885](https://github.com/aws/aws-sdk-go-v2/pull/885))
* Logging support has been added to service clients. See [Logging](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/logging/) in the Developer Guide. ([#872](https://github.com/aws/aws-sdk-go-v2/pull/872))
* `service`: Add support for pre-signed URL clients for S3, RDS, EC2 service ([#888](https://github.com/aws/aws-sdk-go-v2/pull/888))
    * `service/s3`: operations `PutObject` and `GetObject` are now supported with s3 pre-signed url client.
    * `service/ec2`: operation `CopySnapshot` is now supported with ec2 pre-signed url client.
    * `service/rds`: operations `CopyDBSnapshot`, `CreateDBInstanceReadReplica`, `CopyDBClusterSnapshot`, `CreateDBCluster` are now supported with rds pre-signed url client.
* `service/s3`: Add support for S3 access point and S3 on outposts access point ARNs ([#870](https://github.com/aws/aws-sdk-go-v2/pull/870))
* `service/s3control`: Adds support for S3 on outposts access point and S3 on outposts bucket ARNs ([#870](https://github.com/aws/aws-sdk-go-v2/pull/870))

## Migrating from v2 preview SDK's v0.29.0 to v0.30.0

### aws.BuildableHTTPClient move
The `aws`'s `BuildableHTTPClient` HTTP client implementation was moved to `aws/transport/http` as `BuildableClient`. If your application used the `aws.BuildableHTTPClient` type, update it to use the `BuildableClient` in the `aws/transport/http` package.

### Slice and Map API member types
This release includes several code generation updates for API client's slice map members. Using API modeling metadata the Slice and map members are now generated as value types instead of pointer types. For your application this means that for these types, the SDK no longer will have pointer member types, and have value member types.

To migrate to this change you'll need to remove the pointer handling for slice and map members, and instead use value type handling of the member values.

### Boolean and Number API member types
Similar to the slice and map API member types being generated as value, the SDK's code generation now has metadata where the SDK can generate boolean and number members as value type instead of pointer types.

To migrate to this change you'll need to remove the pointer handling for numbers and boolean member types, and instead use value handling.

# Release 2020-10-30

## New Features
* Adds HostnameImmutable flag on aws.Endpoint to direct SDK if the associated endpoint is modifiable.([#848](https://github.com/aws/aws-sdk-go-v2/pull/848))

## Bug Fixes
* Fix SDK handling of xml based services - xml namespaces ([#858](https://github.com/aws/aws-sdk-go-v2/pull/858))
  * Fixes ([#850](https://github.com/aws/aws-sdk-go-v2/issues/850))

## Service Client Highlights
* API Clients have been bumped to version `v0.29.0`
    * Regenerate API Clients from update API models.
* Improve client doc generation.

## Core SDK Highlights
* Dependency Update: Updated SDK dependencies to their latest versions.

## Migrating from v2 preview SDK's v0.28.0 to v0.29.0
* API Clients ResolverOptions type renamed to EndpointResolverOptions

# Release 2020-10-26

## New Features
* `service/s3`: Add support for Accelerate, and Dualstack ([#836](https://github.com/aws/aws-sdk-go-v2/pull/836))
* `service/s3control`: Add support for Dualstack ([#836](https://github.com/aws/aws-sdk-go-v2/pull/836))

## Service Client Highlights
* API Clients have been bumped to version `v0.28.0`
    * Regenerate API Clients from update API models.
* `service/s3`: Add support for Accelerate, and Dualstack ([#836](https://github.com/aws/aws-sdk-go-v2/pull/836))
* `service/s3control`: Add support for Dualstack ([#836](https://github.com/aws/aws-sdk-go-v2/pull/836))
* `service/route53`: Fix sanitizeURL customization to handle leading slash(`/`) [#846](https://github.com/aws/aws-sdk-go-v2/pull/846)
    * Fixes [#843](https://github.com/aws/aws-sdk-go-v2/issues/843)
* `service/route53`: Fix codegen to correctly look for operations that need sanitize url ([#851](https://github.com/aws/aws-sdk-go-v2/pull/851))

## Core SDK Highlights
* `aws/protocol/restjson`: Fix unexpected JSON error response deserialization ([#837](https://github.com/aws/aws-sdk-go-v2/pull/837))
    * Fixes [#832](https://github.com/aws/aws-sdk-go-v2/issues/832)
* `example/service/s3/listobjects`: Add example for Amazon S3 ListObjectsV2 ([#838](https://github.com/aws/aws-sdk-go-v2/pull/838))

# Release 2020-10-16

## New Features
* `feature/s3/manager`:
  * Initial `v0.1.0` release
  * Add the Amazon S3 Upload and Download transfer manager ([#802](https://github.com/aws/aws-sdk-go-v2/pull/802))

## Service Client Highlights
* Clients have been bumped to version `v0.27.0`
* `service/machinelearning`: Add customization for setting client endpoint with PredictEndpoint value if set ([#782](https://github.com/aws/aws-sdk-go-v2/pull/782))
* `service/s3`: Fix empty response body deserialization in case of error response ([#801](https://github.com/aws/aws-sdk-go-v2/pull/801))
  * Fixes xml deserialization util to correctly handle empty response body in case of an error response.
* `service/s3`: Add customization to auto fill Content-Md5 request header for Amazon S3 operations ([#812](https://github.com/aws/aws-sdk-go-v2/pull/812))
* `service/s3`: Add fallback to using HTTP status code for error code ([#818](https://github.com/aws/aws-sdk-go-v2/pull/818))
  * Adds falling back to using the HTTP status code to create a API Error code when not error code is received from the service, such as HeadObject.
* `service/route53`: Add support for deserialzing `InvalidChangeBatch` API error ([#792](https://github.com/aws/aws-sdk-go-v2/pull/792))
* `codegen`: Remove API client `Options` getter methods ([#788](https://github.com/aws/aws-sdk-go-v2/pull/788))
* `codegen`: Regenerate API Client modeled endpoints ([#791](https://github.com/aws/aws-sdk-go-v2/pull/791))
* `codegen`: Sort API Client struct member paramaters by required and alphabetical ([#787](https://github.com/aws/aws-sdk-go-v2/pull/787))
* `codegen`: Add package docs to API client modules ([#821](https://github.com/aws/aws-sdk-go-v2/pull/821))
* `codegen`: Rename `smithy-go`'s `smithy.OperationError` to `smithy.OperationInvokeError`.

## Core SDK Highlights
* `config`:
  * Bumped to `v0.2.0`
  * Refactor Config Module, Add Config Package Documentation and Examples, Improve Overall SDK Readme ([#822](https://github.com/aws/aws-sdk-go-v2/pull/822))
* `credentials`:
  * Bumped to `v0.1.2`
  * Strip Monotonic Clock Readings when Comparing Credential Expiry Time ([#789](https://github.com/aws/aws-sdk-go-v2/pull/789))
* `ec2imds`:
  * Bumped to `v0.1.2`
  * Fix refreshing API token if expired ([#789](https://github.com/aws/aws-sdk-go-v2/pull/789))

## Migrating from v0.26.0 to v0.27.0

#### Configuration

The `config` module's exported types were trimmed down to add clarity and reduce confusion. Additional changes to the `config` module' helpers.

* Refactored `WithCredentialsProvider`, `WithHTTPClient`, and `WithEndpointResolver` to functions instead of structs.
* Removed `MFATokenFuncProvider`, use `AssumeRoleCredentialOptionsProvider` for setting options for `stscreds.AssumeRoleOptions`.
* Renamed `WithWebIdentityCredentialProviderOptions` to `WithWebIdentityRoleCredentialOptions`
* Renamed `AssumeRoleCredentialProviderOptions` to `AssumeRoleCredentialOptionsProvider`
* Renamed `EndpointResolverFuncProvider` to `EndpointResolverProvider`

#### API Client
* API Client `Options` type getter methods have been removed. Use the struct members instead.
* The error returned by API Client operations was renamed from `smithy.OperationError` to `smithy.OperationInvokeError`.

# Release 2020-09-30

## Service Client Highlights
* Service clients have been bumped to `v0.26.0` simplify the documentation experience when using [pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2).
* `service/s3`: Disable automatic decompression of getting Amazon S3 objects with the `Content-Encoding: gzip` metadata header. ([#748](https://github.com/aws/aws-sdk-go-v2/pull/748))
  * This changes the SDK's default behavior with regard to making S3 API calls. The client will no longer automatically set the `Accept-Encoding` HTTP request header, nor will it automatically decompress the gzipped response when the `Content-Encoding: gzip` response header was received.
  * If you'd like the client to sent the `Accept-Encoding: gzip` request header, you can add this header to the API operation method call with the [SetHeaderValue](https://pkg.go.dev/github.com/awslabs/smithy-go/transport/http#SetHeaderValue). middleware helper.
* `service/cloudfront/sign`: Fix cloudfront example usage of SignWithPolicy ([#673](https://github.com/aws/aws-sdk-go-v2/pull/673))
  * Fixes [#671](https://github.com/aws/aws-sdk-go-v2/issues/671) documentation typo by correcting the usage of `SignWithPolicy`.

## Core SDK Highlights
* SDK core module released at `v0.26.0`
* `config` module released at `v0.1.1`
* `credentials` module released at `v0.1.1`
* `ec2imds` module released at `v0.1.1`


# Release 2020-09-28
## Announcements
We’re happy to share the updated clients for the v0.25.0 preview version of the AWS SDK for Go V2.

The updated clients leverage new developments and advancements within AWS and the Go software ecosystem at large since
our original preview announcement. Using the new clients will be a bit different than before. The key differences are:
simplified API operation invocation, performance improvements, support for error wrapping, and a new middleware architecture.
So below we have a guided walkthrough to help try it out and share your feedback in order to better influence the features
you’d like to see in the GA version.

See [Announcement Blog Post](https://aws.amazon.com/blogs/developer/client-updates-in-the-preview-version-of-the-aws-sdk-for-go-v2/) for more details.

## Service Client Highlights
* Initial service clients released at version `v0.1.0`
## Core SDK Highlights
* SDK core module released at `v0.25.0`
* `config` module released at `v0.1.0`
* `credentials` module released at `v0.1.0`
* `ec2imds` module released at `v0.1.0`

## Migrating from v2 preview SDK's v0.24.0 to v0.25.0

#### Design changes

The v2 preview SDK `v0.25.0` release represents a significant stepping stone bringing the v2 SDK closer to its target design and usability. This release includes significant breaking changes to the v2 preview SDK. The updates in the `v0.25.0` release focus on refactoring and modularization of the SDK’s API clients to use the new [client design](https://github.com/aws/aws-sdk-go-v2/issues/438), updated request pipeline (aka [middleware](https://pkg.go.dev/github.com/awslabs/smithy-go/middleware)), refactored [credential providers](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials), and [configuration loading](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) packages.

We've also bumped the minimum supported Go version with this release. Starting with v0.25.0 the SDK requires a minimum version of Go `v1.15`.

As a part of the refactoring done to v2 preview SDK some components have not been included in this update. The following is a non exhaustive list of features that are not available.

* API Paginators - [#439](https://github.com/aws/aws-sdk-go-v2/issues/439)
* API Waiters - [#442](https://github.com/aws/aws-sdk-go-v2/issues/442)
* Presign URL - [#794](https://github.com/aws/aws-sdk-go-v2/issues/794)
* Amazon S3 Upload and Download manager - [#802](https://github.com/aws/aws-sdk-go-v2/pull/802)
* Amazon DynamoDB's AttributeValue marshaler, and Expression package - [#790](https://github.com/aws/aws-sdk-go-v2/issues/790)
* Debug Logging - [#594](https://github.com/aws/aws-sdk-go-v2/issues/594)

We expect additional breaking changes to the v2 preview SDK in the coming releases. We expect these changes to focus on organizational, naming, and hardening the SDK's design for future feature capabilities after it is released for general availability.


#### Relocated Packages

In this release packages within the SDK were relocated, and in some cases those packages were converted to Go modules. The following is a list of packages have were relocated.

* `github.com/aws/aws-sdk-go-v2/aws/external` => `github.com/aws/aws-sdk-go-v2/config` module
* `github.com/aws/aws-sdk-go-v2/aws/ec2metadata` => `github.com/aws/aws-sdk-go-v2/ec2imds` module

The `github.com/aws/aws-sdk-go-v2/credentials` module contains refactored credentials providers.

* `github.com/aws/aws-sdk-go-v2/ec2rolecreds` => `github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds`
* `github.com/aws/aws-sdk-go-v2/endpointcreds` => `github.com/aws/aws-sdk-go-v2/credentials/endpointcreds`
* `github.com/aws/aws-sdk-go-v2/processcreds` => `github.com/aws/aws-sdk-go-v2/credentials/processcreds`
* `github.com/aws/aws-sdk-go-v2/stscreds` => `github.com/aws/aws-sdk-go-v2/credentials/stscreds`


#### Modularization

New modules were added to the v2 preview SDK to allow the components to be versioned independently from each other. This allows your application to depend on specific versions of an API client module, and take discrete updates from the SDK core and other API client modules as desired.

* [github.com/aws/aws-sdk-go-v2/config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config)
* [github.com/aws/aws-sdk-go-v2/credentials](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials)
* Module for each API client, e.g. [github.com/aws/aws-sdk-go-v2/service/s3](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3)


#### API Clients

The following is a list of the major changes to the API client modules

* Removed paginators: we plan to add these back once they are implemented to integrate with the SDK's new API client design.
* Removed waiters: we need to further investigate how the V2 SDK should expose waiters, and how their behavior should be modeled.
* API Clients are now Go modules. When migrating to the v2 preview SDK `v0.25.0`, you'll need to add the API client's module to your application's go.mod file.
* API parameter nested types have been moved to a `types` package within the API client's module, e.g. `github.com/aws/aws-sdk-go-v2/service/s3/types` These types were moved to improve documentation and discovery of the API client, operation, and input/output types. For example Amazon S3's ListObject's operation [ListObjectOutput.Contents](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3/#ListObjectsOutput) input parameter is a slice of [types.Object](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3/types#Object).
* The client operation method has been renamed, removing the `Request` suffix. The method now invokes the operation instead of constructing a request, which needed to be invoked separately. The operation methods were also expanded to include functional options for providing operation specific configuration, such as modifying the request pipeline.

```go
result, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
    TableName: aws.String("exampleTable"),
}, func(o *Options) {
    // Limit operation calls to only 1 attempt.
    o.Retryer = retry.AddWithMaxAttempts(o.Retryer, 1)
})
```


#### Configuration

In addition to the `github.com/aws/aws-sdk-go-v2/aws/external` package being made a module at `github.com/aws/aws-sdk-go-v2/config`, the `LoadDefaultAWSConfig` function was renamed to `LoadDefaultConfig`.

The `github.com/aws/aws-sdk-go-v2/aws/defaults` package has been removed. Its components have been migrated to the `github.com/aws/aws-sdk-go-v2/aws` package, and `github.com/aws/aws-sdk-go-v2/config` module.


#### Error Handling

The `github.com/aws/aws-sdk-go-v2/aws/awserr` package was removed as a part of the SDK error handling refactor. The SDK now uses typed errors built around [Go v1.13](https://golang.org/doc/go1.13#error_wrapping)'s [errors.As](https://pkg.go.dev/errors#As) and [errors.Unwrap](https://pkg.go.dev/errors#Unwrap) features. All SDK error types that wrap other errors implement the `Unwrap` method. Generic v2 preview SDK errors created with `fmt.Errorf` use `%w` to wrap the underlying error.

The SDK API clients now include generated public error types for errors modeled for an API. The SDK will automatically deserialize the error response from the API into the appropriate error type. Your application should use `errors.As` to check if the returned error matches one it is interested in. Your application can also use the generic interface [smithy.APIError](https://pkg.go.dev/github.com/awslabs/smithy-go/#APIError) to test if the API client's operation method returned an API error, but not check against a specific error.

API client errors returned to the caller will use error wrapping to layer the error values. This allows underlying error types to be specific to their use case, and the SDK's more generic error types to wrap the underlying error.

For example, if an [Amazon DynamoDB](https://aws.amazon.com/dynamodb/) [Scan](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb#Scan) operation call cannot find the `TableName` requested, the error returned will contain [dynamodb.ResourceNotFoundException](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb/types#ResourceNotFoundException). The SDK will return this error value wrapped in a couple layers, with each layer adding additional contextual information such as [ResponseError](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/transport/http#ResponseError) for AWS HTTP response error metadata , and [smithy.OperationError](https://pkg.go.dev/github.com/awslabs/smithy-go/#OperationError) for API operation call metadata.

```go
result, err := client.Scan(context.TODO(), params)
if err != nil {
    // To get a specific API error
    var notFoundErr *types.ResourceNotFoundException
    if errors.As(err, &notFoundErr) {
        log.Printf("scan failed because the table was not found, %v",
            notFoundErr.ErrorMessage())
    }

    // To get any API error
    var apiErr smithy.APIError
    if errors.As(err, &apiErr) {
        log.Printf("scan failed because of an API error, Code: %v, Message: %v",
            apiErr.ErrorCode(), apiErr.ErrorMessage())
    }

    // To get the AWS response metadata, such as RequestID
    var respErr *awshttp.ResponseError // Using import alias "awshttp" for package github.com/aws/aws-sdk-go-v2/aws/transport/http
    if errors.As(err, &respErr) {
        log.Printf("scan failed with HTTP status code %v, Request ID %v and error %v",
            respErr.HTTPStatusCode(), respErr.ServiceRequestID(), respErr)
    }

    return err
}
```

Logging an error value will include information from each wrapped error. For example, the following is a mock error logged for a Scan operation call that failed because the table was not found.

> 2020/10/15 16:03:37 operation error DynamoDB: Scan, https response error StatusCode: 400, RequestID: ABCREQUESTID123, ResourceNotFoundException: Requested resource not found


#### Endpoints

The `github.com/aws/aws-sdk-go-v2/aws/endpoints` has been removed from the SDK, along with all exported endpoint definitions and iteration behavior. Each generated API client now includes its own endpoint definition internally to the module.

API clients can optionally be configured with a generic [aws.EndpointResolver](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#EndpointResolver) via the [aws.Config.EndpointResolver](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws#Config.EndpointResolver). If the API client is not configured with a custom endpoint resolver it will defer to the endpoint resolver the client module was generated with.
