# Hydrophone

Hydrophone is the module responsible of sending emails. 
This API sends notifications to users for things like forgotten passwords, initial signup, and invitations. 

## [Unreleased]

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
