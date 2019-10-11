Hydrophone Dev Docs
===

## Configuration

See [.vscode/launch.json.template](../.vscode/launch.json.template) or [env.sh](../env.sh) for examples.

### TIDEPOOL_HYDROPHONE_ENV

Contains configuration about the stack where Hydrophone is running.

### TIDEPOOL_HYDROPHONE_SERVICE

Contains configuration necessary to run Hydrophone as a microservice.

#### mongo

This configuration item is a JSON string that uses the following:
- _connectionString_: the connection string to MongoDB, e.g. mongodb://<<user_personal>>:<<password_personal>>@localhost:27017/confirm?authSource=admin

#### service

#### hydrophone

This configuration item is a JSON string that uses the following:
- _serverSecret_: the secret to be used to connect to shoreline and get server token
- _webUrl_: URL for the links to "Blip" in the emails
- _supportUrl_: URL for the links to "Support" in the emails
- _assetUrl_: where public artefacts needed by emails are present (like images)

#### notifierType
Hydrophone currently support 2 sending methods:
* AWS SES (default)
* SMTP

The mail service is specified in the configuration variable `TIDEPOOL_HYDROPHONE_SERVICE.notifierType`. It accepts `ses` or `smtp` (`ses` by default).  

#### smtpEmail
This configuration item is a JSON string that uses the following:
- _fromAddress_: the email address to be used as the email sender
- _serverAdress_: the smtp server adress
- _user_: (if present) this will be used to authenticate to the smtp server
- _password_: (if present) this will be used to authenticate to the smtp server

#### sesEmail
This configuration item is a JSON string that uses the following:
- _fromAddress_: the email address to be used as the email sender
- _region_: the AWS region to be used for AWS SES service
- _serverEndpoint_: (if present) this will be used to override the AWS SES default endpoint (can be used in conjunction with MockServer for example) 


## Email Template Generation

### Current Practice

When updating the email templates within the root `templates` folder, it's important not to simply make inline changes to the templates there.

For the purpose of ease of development and ongoing template maintainance, we develop these templates with the more common, web-friendly approach of using an external stylesheet and keeping our markup clean.

We then use a tool that _inlines_ the CSS for us in a way that's appropriate for emails.

The goal here is to ensure that we keep the many email templates consistent with each other as far as styling goes, and keep our HTML markup clean.

#### Developing with Source Files

For now, all the source files for development are in the `templates-source` folder sitting alongside this doc.

You can serve these files however you like for local development. One simple solution is to, from the terminal in the `templates-source` directory, run python's SimpleHTTPServer like this:

```shell
python -m SimpleHTTPServer 8000
```

At this point, you should be able to view the email in your browser at, for instance, `http://localhost:8000/signup.html`.

We also have an `index.html` file set up with links to all the templates.

#### Assets (Images) file locations

All the email assets must be stored in a publicly accessible location. We use Amazon S3 buckets for this.  Assets are stored per environment, so we can have different assets on `dev`, `stg`, `int`, and `prd`

The bucket urls follow this pattern:

`https://s3-us-west-2.amazonaws.com/tidepool-[env]-asset/[type]/[file]`

So the logo image for the dev environment may be found at:

`https://s3-us-west-2.amazonaws.com/tidepool-dev-asset/img/tidepool_logo_light_x2.png`

Currently, only the backend engineering team has access to these buckets, so all image change requests should go through the backend engineering team lead.

During development, you should change the image sources to use files in the local `img` folder. This way, you won't need to ask to have the files uploaded to S3 until you're sure they're ready for QA. This is also helpful, as it keeps a record of intended file changes in version control.

#### Inlining the CSS

Until we implement the [Recommended Future Improvements](#recommended-future-improvements) detailed later in this doc, inlining the CSS will be a manual process. We currently use the online [PutsMail CSS Inliner](https://www.putsmail.com/inliner) tool made by the email testing company, Litmus.

To prepare the markup for PutsMail, we need to remove the external stylesheet link to `styles.css` from the `<head>` of each template, and replace it with the actual content of styles.css within proper `<style>` tags.

So, replace

```html
<link rel="stylesheet" type="text/css" href="css/styles.css" />
```

with

```html
<style type="text/css">
  [...contents of styles.css pasted here]
</style>
```

*__IMPORTANT GOTCHA__*: The PutsMail CSS Inliner doesn't properly handle the Go template variables in html attributes, so we need to manually find/replace all occurences of `%20` with an empty space.

So, for instance

```html
<a href="{{%20.WebURL%20}}" />
```

becomes

```html
<a href="{{ .WebURL }}" />
```
#### Local Email Testing

Testing locally requires that you have a temporary AWS SES credentials provide to you by the backend engineering team lead. These credentials must be kept private, as soon as testing is complete, the engineering team lead mush be informed so as to revoke them.

Extreme care must be taken to not commit this to out public git repo. If that were to happen, for any reason or lenght of time, the backend engineering team lead MUST be notified immediately.

#### Multiple Email Client Testing

It's important to test the final email rendering in as many email clients as possible.  Emails are notorioulsy fickle, and using a testing service such as Litmus or Email on Acid is recommended before going to production with any markup/styling changes.

We currently haven't settled on which of these 2 services to set up an account with. We've tried both. Email on Acid is about half the price, and suits our needs well enough, so we will likely go that route. Litmus, however, is nicer for it's in-place editing to iron out the many difficult issues in Outlook (or really any of the MS mail clients).

#### Final Post-Inlining Steps

Once our CSS is inlined properly, there are a couple of things we need to do before pasting the resulting code into the corresponding Go templates.

Any Asset URLs need to be replaced with with the `{{ .AssetURL }}` Go template variable. This allows us to set the appropriate asset url for each environment via build-time config.

For instance, replace
```html
<img src="https://s3-us-west-2.amazonaws.com/tidepool-dev-asset/img/tidepool_logo_light_x2.png" />
or
<img src="img/tidepool_logo_light_x2.png" />
```

with

```html
<img src="{{ .AssetURL }}/img/tidepool_logo_light_x2.png" />
```

### Recommended Future Improvements

For now, what we're doing is better than in-place editing of the templates for the reasons noted above. There are, however, many ways this process could be improved in the future.

The most notable candidate for improvement is to perform the CSS _inlining_ with a local build tool (perhaps Gulp) to avoid relying on a 3rd party online service, and avoid the manual copy/pasting required.

Another would be to share all of the common markup in HTML templates, and piece them together at build time. Again, Gulp could be used for this, and would be rather quick to implement. There is a good writeup [here](https://bitsofco.de/a-gulp-workflow-for-building-html-email/) on one possible approach using gulp. There is even a [github repo](https://github.com/ireade/gulp-email-workflow/tree/master/src/templates) from this example that is meant as a starting point, so we could basically plug our styles and templates in to it and it should be done at that point.

This process would also take care of all of the other small manual final prepartation steps outlined in our current process above.
