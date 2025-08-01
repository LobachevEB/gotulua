package timefunc

import (
	"errors"
	"fmt"
	"gotulua/errorhandlefunc"
	"gotulua/i18nfunc"
	"gotulua/typesfunc"
	"regexp"
	"strings"
	"time"
)

const (
	ToInternalFormat = iota
	ToUserFormat
)

const InternalDateTimeFormat = "yyyymmddhhiiss"
const InternalDateFormat = "yyyymmdd"
const InternalTimeFormat = "hhiiss"
const dtAllowedSymbRegexp = `^[ymdhis./: -]+$`

var DateFormat string = "dd.mm.yyyy"
var TimeFormat string = "hh:ii:ss"
var DateTimeFormat string = "dd.mm.yyyy hh:ii:ss"

func SetDateFormat(df string) {
	err := checkIfDTFormatIsValid(df, typesfunc.TypeDate)
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
	}
	DateFormat = df
}

func SetTimeFormat(tf string) {
	err := checkIfDTFormatIsValid(tf, typesfunc.TypeTime)
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
	}
	TimeFormat = tf
}

func SetDateTimeFormat(dtf string) {
	err := checkIfDTFormatIsValid(dtf, typesfunc.TypeDateTime)
	if err != nil {
		errorhandlefunc.ThrowError(err.Error(), errorhandlefunc.ErrorTypeScript, true)
	}
	DateTimeFormat = dtf
}

func checkIfDTFormatIsValid(df, dtType string) error {
	var errs []error = make([]error, 0)
	if df == "" {
		return errors.New(i18nfunc.T("error.date_format_empty", nil))
	}
	if dtType == typesfunc.TypeDate || dtType == typesfunc.TypeDateTime {
		if !strings.Contains(df, "yy") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_year", nil)))
		}
		if !strings.Contains(df, "yyyyy") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_year_length", nil)))
		}
		if !strings.Contains(df, "mm") && !strings.Contains(df, "MM") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_month", nil)))
		}
		if strings.Contains(df, "mm") && strings.Contains(df, "MM") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_month_duplicate", nil)))
		}
		if strings.Contains(df, "mmm") || strings.Contains(df, "MMM") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_month_length", nil)))
		}
		if !strings.Contains(df, "dd") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_day", nil)))
		}
		if !strings.Contains(df, "ddd") {
			errs = append(errs, errors.New(i18nfunc.T("error.date_format_day_length", nil)))
		}
	}
	if dtType == typesfunc.TypeTime || dtType == typesfunc.TypeDateTime {
		if !strings.Contains(df, "hh") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_hour", nil)))
		}
		if !strings.Contains(df, "yyy") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_hour_length", nil)))
		}
		if !strings.Contains(df, "ii") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_minutes", nil)))
		}
		if strings.Contains(df, "iii") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_minutes_length", nil)))
		}
		if !strings.Contains(df, "ss") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_seconds", nil)))
		}
		if !strings.Contains(df, "sss") {
			errs = append(errs, errors.New(i18nfunc.T("error.time_format_seconds_length", nil)))
		}
	}
	err := errors.Join(errs...)
	return err
}

func checkNoExtraSymbolsInTemplate(tpl string) bool {
	dtPattern := regexp.MustCompile(dtAllowedSymbRegexp)
	return dtPattern.MatchString(tpl)
}

func customTemplateToGoTemplate(ct, mode string) (string, error) {
	if mode != typesfunc.TypeDate && mode != typesfunc.TypeTime && mode != typesfunc.TypeDateTime {
		return "", fmt.Errorf(i18nfunc.T("error.datetime_type", map[string]interface{}{
			"Date":     DateFormat,
			"Time":     TimeFormat,
			"DateTime": DateTimeFormat,
		}))
	}
	var err error
	var gdt, gtt, t string
	if !checkNoExtraSymbolsInTemplate(ct) {
		return "", fmt.Errorf(i18nfunc.T("error.datetime_symbols", map[string]interface{}{
			"Symbols": "0-9./: -",
		}))
	}
	t = ct
	if mode == typesfunc.TypeDate || mode == typesfunc.TypeDateTime {
		gdt = t
		gdt = strings.Replace(gdt, "yyyy", "2006", 1)
		if gdt == t {
			gdt = strings.Replace(gdt, "yy", "06", 1)
		}
		if gdt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_year_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		gdt = strings.Replace(gdt, "mm", "01", 1)
		if gdt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_month_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		gdt = strings.Replace(gdt, "dd", "02", 1)
		if gdt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_day_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		if mode == typesfunc.TypeDate && err != nil {
			return "", err
		}
		if mode == typesfunc.TypeDate {
			return gdt, nil
		}
		t = gdt
	}
	if mode == typesfunc.TypeTime || mode == typesfunc.TypeDateTime {
		gtt = t
		gtt = strings.Replace(gtt, "hh", "15", 1)
		if gtt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_hour_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		gtt = strings.Replace(gtt, "ii", "04", 1)
		if gtt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_minutes_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		gtt = strings.Replace(gtt, "ss", "05", 1)
		if gtt == t {
			err = errors.Join(fmt.Errorf(i18nfunc.T("error.datetime_seconds_missing", map[string]interface{}{
				"Template": ct,
			})))
		}
		if err != nil {
			return "", err
		}
	}
	return gtt, nil
}

func TemplateToRegexp(tpl string) string {
	t := strings.Replace(tpl, "dd", "\\d\\d", 1)
	t1 := t
	t = strings.Replace(t, "yyyy", "\\d\\d\\d\\d", 1)
	if t == t1 {
		t = strings.Replace(t, "yy", "\\d\\d", 1)
	}
	t = strings.Replace(t, "mm", "\\d\\d", 1)
	t = strings.Replace(t, "hh", "\\d\\d", 1)
	t = strings.Replace(t, "ii", "\\d\\d", 1)
	t = strings.Replace(t, "ss", "\\d\\d", 1)
	return t
}

func TemplateToPlaceholder(tpl string) string {
	t := strings.Replace(tpl, "dd", "  ", 1)
	t1 := t
	t = strings.Replace(t, "yyyy", "    ", 1)
	if t == t1 {
		t = strings.Replace(t, "yy", "  ", 1)
	}
	t = strings.Replace(t, "mm", "  ", 1)
	t = strings.Replace(t, "hh", "  ", 1)
	t = strings.Replace(t, "ii", "  ", 1)
	t = strings.Replace(t, "ss", "  ", 1)
	return t
}

func FormatDateTime(inp, mode string, direction int) (string, error) {
	if mode != typesfunc.TypeDate && mode != typesfunc.TypeTime && mode != typesfunc.TypeDateTime {
		return "", fmt.Errorf(i18nfunc.T("error.datetime_type", map[string]interface{}{
			"Date":     DateFormat,
			"Time":     TimeFormat,
			"DateTime": DateTimeFormat,
		}))
	}
	if inp == "" {
		return "", nil
	}
	var fmts, fmtd string
	switch mode {
	case typesfunc.TypeDate:
		if direction == ToInternalFormat {
			fmts = DateFormat
			fmtd = InternalDateFormat
		} else {
			fmts = InternalDateFormat
			fmtd = DateFormat
		}
	case typesfunc.TypeTime:
		if direction == ToInternalFormat {
			fmts = TimeFormat
			fmtd = InternalTimeFormat
		} else {
			fmts = InternalTimeFormat
			fmtd = TimeFormat
		}
	case typesfunc.TypeDateTime:
		if direction == ToInternalFormat {
			fmts = DateTimeFormat
			fmtd = InternalDateTimeFormat
		} else {
			fmts = InternalDateTimeFormat
			fmtd = DateTimeFormat
		}
	}
	gs, err := customTemplateToGoTemplate(fmts, mode)
	if err != nil {
		return "", err
	}
	t, err := time.Parse(gs, inp)
	if err != nil {
		return "", err
	}
	gd, err := customTemplateToGoTemplate(fmtd, mode)
	if err != nil {
		return "", err
	}
	ret := t.Format(gd)
	return ret, nil
}

func CheckDateTimeConsistent(inp, mode string, direction int) error {
	if mode != typesfunc.TypeDate && mode != typesfunc.TypeTime && mode != typesfunc.TypeDateTime {
		return fmt.Errorf(i18nfunc.T("error.datetime_type", map[string]interface{}{
			"Date":     DateFormat,
			"Time":     TimeFormat,
			"DateTime": DateTimeFormat,
		}))
	}
	if inp == "" {
		return nil
	}
	var fmt string
	switch mode {
	case typesfunc.TypeDate:
		if direction == ToInternalFormat {
			fmt = InternalDateFormat
		} else {
			fmt = DateFormat
		}
	case typesfunc.TypeTime:
		if direction == ToInternalFormat {
			fmt = InternalTimeFormat
		} else {
			fmt = TimeFormat
		}
	case typesfunc.TypeDateTime:
		if direction == ToInternalFormat {
			fmt = InternalDateTimeFormat
		} else {
			fmt = DateTimeFormat
		}
	}
	g, err := customTemplateToGoTemplate(fmt, mode)
	if err != nil {
		return err
	}
	_, err = time.Parse(g, inp)
	if err != nil {
		return err
	}
	return nil
}

func Date() string {
	g, err := customTemplateToGoTemplate(DateFormat, typesfunc.TypeDate)
	if err != nil {
		return ""
	}
	return time.Now().Format(g)
}

func Time() string {
	g, err := customTemplateToGoTemplate(TimeFormat, typesfunc.TypeTime)
	if err != nil {
		return ""
	}
	return time.Now().Format(g)
}

func DateTime() string {
	g, err := customTemplateToGoTemplate(DateTimeFormat, typesfunc.TypeDateTime)
	if err != nil {
		return ""
	}
	return time.Now().Format(g)
}

func DateDiff(start, end, mode string) int64 {
	if start == "" || end == "" {
		return -1
	}
	gs, err := customTemplateToGoTemplate(DateFormat, typesfunc.TypeDate)
	if err != nil {
		return -1
	}
	startT, err := time.Parse(gs, start)
	if err != nil {
		return -1
	}
	endT, err := time.Parse(gs, end)
	if err != nil {
		return -1
	}
	switch mode {
	case "d", "D":
		return int64(endT.Sub(startT).Hours() / 24)
	case "w", "W":
		return int64(endT.Sub(startT).Hours() / 168)
	case "m", "M":
		return int64(endT.Sub(startT).Hours() / 720)
	case "y", "Y":
		return int64(endT.Sub(startT).Hours() / 8760)
	}
	return -1
}

func TimeDiff(start, end, mode string) int64 {
	if start == "" || end == "" {
		return -1
	}
	gs, err := customTemplateToGoTemplate(TimeFormat, typesfunc.TypeTime)
	if err != nil {
		return -1
	}
	startT, err := time.Parse(gs, start)
	if err != nil {
		return -1
	}
	endT, err := time.Parse(gs, end)
	if err != nil {
		return -1
	}
	switch mode {
	case "h", "H":
		return int64(endT.Sub(startT).Hours())
	case "m", "M":
		return int64(endT.Sub(startT).Minutes())
	case "s", "S":
		return int64(endT.Sub(startT).Seconds())
	}
	return -1
}

func DateAdd(date string, year, month, day int) string {
	if date == "" {
		return ""
	}
	gs, err := customTemplateToGoTemplate(DateFormat, typesfunc.TypeDate)
	if err != nil {
		return ""
	}
	t, err := time.Parse(gs, date)
	if err != nil {
		return ""
	}
	return t.AddDate(year, month, day).Format(gs)
}

func TimeAdd(t string, hour, minute, second int) string {
	if t == "" {
		return ""
	}
	gs, err := customTemplateToGoTemplate(TimeFormat, typesfunc.TypeTime)
	if err != nil {
		return ""
	}
	tRes, err := time.Parse(gs, t)
	if err != nil {
		return ""
	}
	return tRes.Add(time.Hour*time.Duration(hour) + time.Minute*time.Duration(minute) + time.Second*time.Duration(second)).Format(gs)
}
