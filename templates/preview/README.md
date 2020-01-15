hydrophone email preview service
==========

This http service can be used by translator (and/or email template designers) to preview the various emails templates built from fake data and real localization files.  

## Building
To build the service you simply need to execute the go build command in this directory:  

```
$ cd templates/preview
$ go build
```
This will automatically get the dependencies and build the code. 


## Executing
To start the service simply load the configuration (environment variables available in config.sh) and start the program:

```
$ cd templates/preview
$ . ./config.sh
$ ./preview.exe
``` 

The webserver starts and listen on port 8088.  
The webpage is then available on http://localhost:8088