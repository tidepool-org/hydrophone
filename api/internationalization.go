package api

import (
	"net/http"
	"strconv"
	"strings"
)

type LangQ struct {
	Lang string
	Q    float64
}

const (
	HEADER_LANGUAGE = "Accept-Language"
	USER_LANGUAGE   = "x-tidepool-language"
)

//GetUserChosenLanguage returns the chosen language passed as a custom header
func GetUserChosenLanguage(req *http.Request) string {
	if userlng := req.Header.Get(USER_LANGUAGE); userlng == "" {
		return ""
	} else {
		return userlng
	}
}

//GetBrowserPreferredLanguage returns the preferred language extracted from the request browser
func GetBrowserPreferredLanguage(req *http.Request) string {

	if acptlng := req.Header.Get(HEADER_LANGUAGE); acptlng == "" {
		return ""
	} else if languages := parseAcceptLanguage(acptlng); languages == nil {
		return ""
	} else {
		// if at least 1 lang is found, we return the 2 first characters of the first lang
		// this header language item, although initially made for handling language is sometimes used to handle complete locale under form language-locale (eg FR-fr)
		// hence we take only the 2 first characters
		return languages[0].Lang[0:2]
	}
}

//parseAcceptLanguage will return array of languages extracted from given Accept-Language value
//Accept-Language value is retrieved from the Request Header
func parseAcceptLanguage(acptLang string) []LangQ {
	var lqs []LangQ

	langQStrs := strings.Split(acptLang, ",")
	for _, langQStr := range langQStrs {
		trimedLangQStr := strings.Trim(langQStr, " ")

		langQ := strings.Split(trimedLangQStr, ";")
		if len(langQ) == 1 {
			lq := LangQ{langQ[0], 1}
			lqs = append(lqs, lq)
		} else {
			qp := strings.Split(langQ[1], "=")
			q, err := strconv.ParseFloat(qp[1], 64)
			if err != nil {
				panic(err)
			}
			lq := LangQ{langQ[0], q}
			lqs = append(lqs, lq)
		}
	}
	return lqs
}
