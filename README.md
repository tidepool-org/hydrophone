hydrophone
==========

[![Build Status](https://travis-ci.org/tidepool-org/hydrophone.png)](https://travis-ci.org/tidepool-org/hydrophone)

This API sends notifications (using relevant language) to users for things like forgotten passwords, initial signup, and invitations.  

## Building
To build the Hydrophone module you simply need to execute the build script:  

```
$ ./build.sh
```
This will automatically get the dependencies (using goget) and build the code. 


## Running the Tests
If you would like to contribute then you will likely need to run the tests locally before pushing your changes. 
To run **all** tests you can simply execute the test script from your favorite shell:

`$ ./test.sh`  

To run the tests for a particular folder (i.e. the api part) you need to go into this folder and execute the gotest command:  
To run all tests for this repo then in the root directory use:

```
$ cd ./api
$ gotest
```

## Testing with docker-compose 

Hydrophone, running locally on your machine, can be tested with all othere services running in docker compose by making some small changes in docker-compose.yml and making changes in local /etc/host

Here is the change that has to be done in docker-compose.yml, so that styx can redirect request to the service running on your localhost: 
```
HYDROPHONE_HOST=host.docker.internal
# HYDROPHONE_HOST=hydrophone
```

and then add the floowing hakken line in your local host:
- Windows: C:\Windows\System32\drivers\etc\hosts
- Linux: /etc/hosts

```
127.0.0.1   hakken
```

## Config
The configuration is provided to Hydrophone via 2 environment variables: `TIDEPOOL_HYDROPHONE_ENV` and `TIDEPOOL_HYDROPHONE_SERVICE`.  
The script `env.sh` provided in this repo will set all the necessary variables with default values, allowing you to work on your development environment. However when deploying on another environment, or when using docker you will likely need to change these variables to match your setup.  

## Notes on email customization and internationalization
More information on this in [docs/README.md](docs/README.md)

The emails sent by Hydrophone can be customized and translated in the user's language.  
The templates and locales files are located under /templates:
* /templates/html: html template files
* /templates/locales: content in various languages
* /templates/meta: email structure

**Configuration note:** you do need to provide the template location to Hydrophone in the environment variables as an absolute path. relative path won't work.  
For example:  
```
export TIDEPOOL_HYDROPHONE_SERVICE='{
    "hydrophone" : {
        ...
        "i18nTemplatesPath": "/var/data/hydrophone/templates"
    },
...
}'
```
