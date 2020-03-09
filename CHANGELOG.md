# Hydrophone

Hydrophone is the module responsible for sending emails.
This API sends notifications to users for things like forgotten passwords, initial signup, and invitations.

## Unreleased
### Added
- PT-899: Preview emails
- PT-995: Document the api using openApi and swaggo

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

## dblp.0.3.0 - 2019-03-21

### Added
- PT-232 New api route to send information message to patients when automatically created.
- Change version of GO engines from 1.9.2 to 1.10.2 to align all versions with Dockerfile's

## dblp 0.2.2 - 2019-03-12

### Changed
- Review Look & Feel for Diabeloop

## dblp.0.2.1

### Changed
- PT-117 Review hydrophone emails support link

## dblp.0.2.0

### Added
- PT-156 Don't allow a patient to reset his password
- Add I18n framework to dblp
- Diabeloop Look & Feel

## dblp.0.1.2

### Changed
- Include fix for rsync on Dockerfile

## dblp.0.1.1

### Added
- Add internationalization to hydrophone emails

## dblp.0.1.a

### Added
- Add multi-language
