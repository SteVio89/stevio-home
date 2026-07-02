package common

import "regexp"

// LocaleCodeRe validates locale codes (e.g. "de", "en", "pt_br").
var LocaleCodeRe = regexp.MustCompile(`^[a-z]{2,5}(_[a-z]{2,5})?$`)

// TranslationKeyRe validates UI translation keys.
var TranslationKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_.]*$`)
