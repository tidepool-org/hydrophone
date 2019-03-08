Hydrophone Dev Docs
===

# HTML files templates and Internationalization

## Notice

Internationalization of emails has been introduced in Hydrophone through the use of static HTML files that contain placeholders for localization content to be filled at runtime.
This internationalization is based on the audience language. The audience language following a logic based on Tidepool user language, browser language and English as a default.

As a matter of fact, the previous logic of having in-code templates (ie in .go files) for emails has been moved to a logic of having templates generated from static files residing on the file system. A potential evolution can be to have files hosted on a S3 bucket (after pitfall described below is solved).

The framework needs a specific folder to be on the filesystem and referenced by the environment variable `TIDEPOOL_HYDROPHONE_SERVICE` (_internationalizationTemplatesPath_). This folder contains the following subfolders:
* html: html template files. They are the final ones, with CSS inlined
* locales: content in various languages. One file per language that name is under format {language_ISO2}.yml
* meta: emails structure files
* source: all the HTML artefacts (html, csss, img) to build the final html templates (process of inlining)

## Meta files

Each HTML file has its corresponding meta file. This meta file describes the template structure. One should ensure meta file contains:
- templateFileName: the name of the html file that this meta is linked to (this is the actual way of linking HTML and meta)
- subject: the name of the key for the email subject that has its corresponding values translated in the locale files
- contentParts: an array of all the keys that can be localized. The keys name the placeholders found in the html files under form {{.keyName}}
- escapeParts: an array of key names that will be escaped during localizations. There will be no tentative to replace these keys by a localized value. It will then not be taken by the translation engine. This key will instead be replaced by information given programmatically. A good example is if you want to include the name of the user in the middle of a localizable text. Note: these keys cannot be changed without a code change.

## Pitfall

 Following the previous logic of having all the templates in memory when the service is starting, this first version of emails based on HTML templates has the same pitfall. It needs a service restart to take changes in the HTML files into consideration. 

One part of the path to have a more dynamic behaviour is already crossed  with the use of the meta files. These meta files ensure:
- we can add more content to the html file without code change
- we can change the names of the HTML files without code change
The current limit, besides the pitfall of having to restart service when template is amended, is when a new type of template is needed (other than existing "signup", "password forget", ...): this would require code change.

## Possible Enhancements

The current logic is to have all the templates loaded at the initialization of the API service. This makes the dynamic behaviour of using static files very limited. Review this logic to have static files be monitored and reloaded whenever a change appears.

Have the templates files hosted in an external repository (eg AWS S3) for ease of changes for non-technical teams.

It is needed to resolve the pitfall above before adding this feature.

# Email Template Generation

## Current Practice

The process of updating the HTML templates is NOT to amend the files within the `templates/html` folder. The process is to amend the HTML files in the `templates/source` folder along with the css, then inline the CSS in all the HTML files.

For the purpose of ease of development and ongoing template maintainance, we develop these templates with the more common, web-friendly approach of using an external stylesheet and keeping our markup clean.

We then use a tool that _inlines_ the CSS for us in a way that's appropriate for emails.

The goal here is to ensure that we keep the many email templates consistent with each other as far as styling goes, and keep our HTML markup clean.

## Developing with Source Files

For now, all the source files for development are in the `templates/source` folder.

You can serve these files however you like for local development. One simple solution is to, from the terminal in the `templates-source` directory, run python's SimpleHTTPServer like this:

```shell
# Python 2
python -m SimpleHTTPServer 8000
# Python 3
python -m http.server 8000
```

At this point, you should be able to view the email in your browser at, for instance, `http://localhost:8000/signup.html`.

We also have an `index.html` file set up with links to all the templates, `http://localhost:8000/index.html`.

## Assets (Images) file locations

All the email assets must be stored in a publicly accessible location. We use Amazon S3 buckets for this. Assets are stored in `https://s3-eu-west-1.amazonaws.com/com.diabeloop.public-assets/`.

## Inlining the CSS

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

## Final Post-Inlining Steps

Once our CSS is inlined properly, there are a couple of things we need to do before pasting the resulting code into the corresponding Go templates (`templates/html`).

Any Asset URLs need to be replaced with with the `{{ .AssetURL }}` Go template variable. This allows us to set the appropriate asset url for each environment via build-time config.

For instance, replace
```html
<img src="[https://s3-eu-west-1.amazonaws.com/com.diabeloop.public-assets/img/facebook.png" />
```

with

```html
<img src="{{ .AssetURL }}/img/facebook.png" />
```

# Testing

## Local Email Testing

Testing locally requires that you have a temporary AWS SES credentials provide to you by the backend engineering team lead. These credentials must be kept private, as soon as testing is complete, the engineering team lead mush be informed so as to revoke them.

Extreme care must be taken to not commit this to out public git repo. If that were to happen, for any reason or lenght of time, the backend engineering team lead MUST be notified immediately.

## Multiple Email Client Testing

It's important to test the final email rendering in as many email clients as possible.  Emails are notorioulsy fickle, and using a testing service such as Litmus or Email on Acid is recommended before going to production with any markup/styling changes.

We currently haven't settled on which of these 2 services to set up an account with. We've tried both. Email on Acid is about half the price, and suits our needs well enough, so we will likely go that route. Litmus, however, is nicer for it's in-place editing to iron out the many difficult issues in Outlook (or really any of the MS mail clients).

# Recommended Future Improvements

For now, what we're doing is better than in-place editing of the templates for the reasons noted above. There are, however, many ways this process could be improved in the future.

The most notable candidate for improvement is to perform the CSS _inlining_ with a local build tool (perhaps Gulp) to avoid relying on a 3rd party online service, and avoid the manual copy/pasting required.

Another would be to share all of the common markup in HTML templates, and piece them together at build time. Again, Gulp could be used for this, and would be rather quick to implement. There is a good writeup [here](https://bitsofco.de/a-gulp-workflow-for-building-html-email/) on one possible approach using gulp. There is even a [github repo](https://github.com/ireade/gulp-email-workflow/tree/master/src/templates) from this example that is meant as a starting point, so we could basically plug our styles and templates in to it and it should be done at that point.

This process would also take care of all of the other small manual final preparation steps outlined in our current process above.

