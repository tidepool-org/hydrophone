hydrophone email preview service
==========

This http service can be used by translators (and/or email template designers) to preview the various emails templates built from fake data and real localization files.  

## Building
To build the service you simply need to execute the go build command in this directory:  

```
$ cd templates/preview
$ go build
```
This will automatically get the dependencies and build the code. 


## Executing
To start the service you need to edit the configuration file (local-env.sh), source it and then start the program:

```
$ cd templates/preview
$ . ./local-env.sh
$ ./preview
``` 

The webserver starts and listen on port 8088.  
The webpage is then available on http://localhost:8088  
Crowdin live preview page is available on http://localhost:8088/live_preview

