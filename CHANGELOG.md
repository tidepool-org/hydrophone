# Hydrophone

Hydrophone is the module responsible for sending emails.
This API sends notifications to users for things like forgotten passwords, initial signup, and invitations.

## UNRELEASED
### Fixed
- YLP-975: support email link is not correctly rendered on email "patient password info"

### Engineering
- YLP-691: consolidate duplicate strings in locales
- YLP-213: cleanup build/artifact scripts

## 1.7.1 - 2021-08-02
### Fixed
- YLP-908: verify the type of confirmation request when validating a signup (potential security issue)
- YLP-907: escape html characters from dynamic values

### Engineering
- Dockerise Hydromail so it can be deployed in k8s environments
- YLP-879: SES monitoring: enable SES configuration set and tags in SES emails

## 1.7.0 - 2021-07-01
### Engineering
- YLP-867: Create a new route to cancel invites sent to a specific email address.

## 1.6.7 - 2021-06-15
### Fixed
- YLP-802: Handle special case when a team invite is sent to someone who is not yet registered

## 1.6.6 - 2021-06-14
### Engineering
- YLP-699: Add CodeQL Analysis 

## 1.6.5 - 2021-06-10
### Fixed
- YLP-802: Handle special case when a team invite is sent to someone who is not yet registered
- YLP-795: Set invitation language with the sender's language when the invitee's language is not known
- YLP-778: A patient shouldn't be able to add another patient as a caregiver

### Engineering
- YLP-804 Implement hydrophone client for harbour

## 1.6.4 - 2021-05-20
### Fixed
- It is not allowed to send multiple team invite to the same patient (duplicates)

### Added
- New generic route to cancel invites (caregiver and team invites)

## 1.6.3
### Fixed 
- Encoded emails are missing in email templates.
- Hydromail: correct the preview of new emails (caregiver invitation)

## 1.6.2 
### Changed
- YLP-682 Add team information in received invitations

### Fixed 
- YLP-709 Bug: get the correct ID to dismiss an invite
- YLP-708 A HCP user shouldn't invite a patient to join a team and vice-versa
- YLP-743 A team invitation for a patient is valid if the patient account exists

## 1.6.1 - 2021-05-04
### Changed
- YLP-710 Modify route /accept/team/invite to acknowledge notifications that does not requires a specific action (i.e. change member role)
### Fixed 
- YLP-531 Bug: Reset key can be null on a reset password api call

## 1.6.0 - 2021-04-28
### Changed
- YLP-559 emails which are part of URLs are URL encoded
- YLP-516 invitations to join a team
### Fixed
- YLP-532 Reset password demand can be used several times
- YLP-705 Hydrophone should consider caregivers as clinical accounts on pin-reset/forgot-emails and signup

## 1.5.1 - 2021-03-12
### Engineering Use
- Switch CI build to Jenkins (instead of Travis)

## 1.5.0 - 2021-03-09
### Changed
- YLP-472 Switch permission client from gatekeeper to crew
- YLP-447 Upgrade go-common to 0.6.2 version
- YLP-516 Manage invitations to join a team (invite, add admin role, delete member)
- YLP-444 Update hydrophone emails style
## 1.4.0 - 2021-01-11
### Changed
- Add Italian and Spanish locales

## 1.3.0 - 2020-11-05
### Changed
- YLP-263 Accept a language as a header parameter for sending "signup" and "forgot pwd" emails
### Engineering
- Review buildDoc to ensure copy latest is done

## 1.2.2 - 2020-10-29
### Engineering
- YLP-242 Review openapi generation so we can serve it through a website
- Update to Go 1.15

## 1.2.1 - 2020-09-25
### Fixed
- Fix S3 deployment

## 1.2.0 - 2020-09-16
### Changed
- PT-1440 Hydrophone should be able to start without MongoDb

## 1.1.0 - 2020-08-27
### Changed
- PT-417 Update German translations

## 1.0.1 - 2020-08-04
### Engineering
- PT-1445 Generate SOUP document

## 1.0.0 - 2020-07-30
### Changed
- PT-1410 Password reset: distinguish information message and reset message containing key for patient
- PT-1412 Update hydrophone email layout and strings

### Engineering
- YLP-48 change crowdin live-preview pseudo language (use chr instead of it)

## 0.10.1 - 2020-07-06
### Fixed 
- PT-1406 Update the format of the message sent to patient when requesting a password reset
### Engineering
- Review the OpenAPI documentation that was not correct on the security part

## 0.10.0 - 2020-07-02
### Added
- PT-1315: Generate and Send an email with a new handset PIN code
### Changed
- PT-1382: Reduce activation key length for password reset

## 0.9.0 - 2020-05-18
### Changed
- PT-1278 Integrate Tidepool master for hydrophone
- PT-1341 Change signup link when invited

## 0.8.1 - 2020-04-07
### Engineering use
- PT-1214 integrate Crowdin live preview in HydroMail (email template preview webapp)
- PT-1199 Remove Highwater (replace by log audits)

## 0.8.0 - 2020-03-30
### Added
- PT-899: Preview emails
- PT-995: Document the api using openApi and swaggo
### Changed
- PT-414 Add switch to enable/disable sending of emails

### Changed
- PT-1139: Add openapi generation and deployment in pipeline

## 0.7.0 - 2019-10-18
### Added
- PT-671: Display the application version number on the status endpoint (/status).  
  This depends on go-common v0.3.0
### Changed
- Switch from dep to go modules to manage go dependencies.
- Upgrade to GO 1.12.7

## 0.6.0 - 2019-10-11
### Added
- PT-175: implement an STMP notifier to offer an alternative to aws ses.

## 0.5.0 - 2019-10-02
### Added
- PT-636 Add a new route for sanity check email to ensure configuration is allowing emails to actually be sent

### Fixed
- PT-311 Hydrophone service return HTTP 200 when the SES email service returned an error

## 0.4.0 - 2019-07-31
### Added
- Add capacity to override the AWS SES endpoint through the environment variable TIDEPOOL_HYDROPHONE_SERVICE/sesEMail
- Integrate Tidepool latest changes

  __!!! There are changes in the way the AWS credentials are challenged !!!__ (see [docs/README.md](docs/README.md) for more information on this)

### Changed
- Review AWS SES Errors handling
- Update to MongoDb 3.6 drivers in order to use replica set connections

## 0.3.3 - 2019-06-28
### Fixed
- PT-449 Fix Error when several invitations are sent to a person who does not have an account yet. The first invitation can be accepted but the remaining ones cannot be.

## 0.3.2 - 2019-04-17

### Changed
- Fix status response of the service. On some cases (MongoDb restart mainly) the status was in error whereas all other entrypoints responded.

## 0.3.1 - 2019-04-09

### Changed
- PT-301 Fix wrong link to detailed instructions for Patient Reset Password. Complete link is now entirely in configuration
- Support URL has been changed in base configuration to be a mailto instead of https website
- PT-305 Fix issue: email is not sent when confirmation does not exist in database.

## 0.3.0 - 2019-03-21

### Added
- PT-232 New api route to send information message to patients when automatically created.
- Change version of GO engines from 1.9.2 to 1.10.2 to align all versions with Dockerfile's

## 0.2.2 - 2019-03-12

### Changed
- Review Look & Feel for Diabeloop

## 0.2.1

### Changed
- PT-117 Review hydrophone emails support link

## 0.2.0

### Added
- PT-156 Don't allow a patient to reset his password
- Add I18n framework to dblp
- Diabeloop Look & Feel

## 0.1.2

### Changed
- Include fix for rsync on Dockerfile

## 0.1.1

### Added
- Add internationalization to hydrophone emails

## 0.1.0

### Added
- Add multi-language
