package templates

import "./../models"

const _SignupCustodialClinicSubjectTemplate string = `Diabetes Clinic Follow Up - Claim Your Account`
const _SignupCustodialClinicBodyTemplate string = `
<html>
  <head>
    <meta name='viewport' content='width=device-width'/>
    <meta http-equiv='Content-Type' content='text/html; charset=UTF-8'/>
    <title>Diabetes Clinic Follow Up - Claim Your Account</title>
    <link href='http://fonts.googleapis.com/css?family=Nunito:400,300,700' rel='stylesheet' type='text/css'>
  </head>

  <body style='background-color: #FFFFFF'>

    <div class='container' style='background-color:#F5F5F5; padding:20px; margin:0 auto; max-width:500px'>
      <div align='center' style='padding:10px 10px 0; margin:0;'>
        <a href='https://www.tidepool.org'><img width='185.348837209' height='20' src='http://drive.google.com/uc?export=view&id=0BwI0YrjnbmtXYkdQS0xqaThyTGc'/></a>
      </div>

      <br>

      <div align='center'>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 14px; color:#000; line-height:1.1; padding:15px 0 15px; margin:0;'>Hi, {{ .FullName }}!</p>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 14px; color:#000; line-height:1.1; padding:0 0 30px; margin:0;'>{{ .CreatorName }} created a Tidepool account for your diabetes device data. You can take ownership of your free account to view and upload data from home.</p>
      </div>

      <br>

      <div align='center' style='padding:0;'>
        <a style='background-color:#627CFB; font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:400; font-size: 14px; color:#FFFFFF; padding:10px 20px; margin:0; border-radius:20px; text-decoration: none;' href='{{ .BlipURL }}/login?signupEmail={{ .Email }}&signupKey={{ .Key }}'>Claim Your Account</a>
      </div>

      <br>

      <div align='center' style='padding:0 60px 0; margin:0'>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 14px; color:#000; line-height:1.1; padding:30px 0 15px; margin:0;'>To upload new data, get the <a href='https://chrome.google.com/webstore/detail/tidepool-uploader/cabklgajffclbljkhmjphejemhpbghfb'>Tidepool Uploader</a>. It works on Mac and PC, using the Chrome browser.</p>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 14px; color:#000; line-height:1.1; padding:0 0 15px; margin:0;'>For Dexcom + iPhone users, upload automatically using Tidepool’s mobile app, <a href='https://itunes.apple.com/us/app/blip-notes/id1026395200?mt=8'>Blip Notes</a>.</p>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 14px; color:#000; line-height:1.1; padding:0 0 15px; margin:0;'>If you have any questions, reach out to us at support@tidepool.org.</p>
      </div>

      <div align='center'>
        <p style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-style: italic; font-size: 13px; color:#000; line-height:1.1; padding:30px 0 15px; margin:0;'>Tidepool is a non-profit company with the mission of making diabetes easier for you and your clinician. Our software is free for you and your care team, forever. We fundamentally believe that you own your data and will never do anything with it without your explicit permission.</p>
      </div>

      <div align='center' style='font-family: Nunito, sans-serif, Helvetica Neue, Helvetica; font-weight:300; font-size: 12px; color:#444; line-height:1.8; padding:5px 0 0 0; margin:0;'>
        <a style='margin:0; display:block; text-decoration:none; color:#444' href='https://www.tidepool.org'>tidepool.org</a>
      </div>
    </div>
  </body>
</html>
`

func NewSignupCustodialClinicTemplate() (models.Template, error) {
	return models.NewPrecompiledTemplate(models.TemplateNameSignupCustodialClinic, _SignupCustodialClinicSubjectTemplate, _SignupCustodialClinicBodyTemplate)
}
