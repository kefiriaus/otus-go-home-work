package hw09structvalidator

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	tagName    = "validate"
	nestedRule = "nested"
)

var (
	ErrNotStruct        = errors.New("value is not a struct")
	ErrUnsupportedType  = errors.New("unsupported field type")
	ErrInvalidRule      = errors.New("malformed validation rule")
	ErrUnknownRule      = errors.New("unknown validation rule")
	ErrInvalidRuleParam = errors.New("invalid validation rule parameter")
)

var (
	ErrLen    = errors.New("invalid length")
	ErrRegexp = errors.New("does not match regexp")
	ErrIn     = errors.New("not in allowed set")
	ErrMin    = errors.New("less than allowed minimum")
	ErrMax    = errors.New("greater than allowed maximum")
)

type ValidationError struct {
	Field string
	Err   error
}

func (v ValidationError) Error() string {
	return v.Field + ": " + v.Err.Error()
}

func (v ValidationError) Unwrap() error {
	return v.Err
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v))
	for _, e := range v {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}

func Validate(v interface{}) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return fmt.Errorf("%w: got nil", ErrNotStruct)
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("%w: got %T", ErrNotStruct, v)
	}

	rt := rv.Type()
	var validationErrs ValidationErrors

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tag, ok := field.Tag.Lookup(tagName)
		if !ok || tag == "" || tag == "-" {
			continue
		}

		if tag == nestedRule {
			nested, err := validateNested(field.Name, rv.Field(i))
			if err != nil {
				return err
			}
			validationErrs = append(validationErrs, nested...)
			continue
		}

		fieldErrs, err := validateField(rv.Field(i), tag)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
		for _, fe := range fieldErrs {
			validationErrs = append(validationErrs, ValidationError{Field: field.Name, Err: fe})
		}
	}

	if len(validationErrs) == 0 {
		return nil
	}
	return validationErrs
}

func validateField(fv reflect.Value, tag string) ([]error, error) {
	typ := fv.Type()
	kind := typ.Kind()

	if kind == reflect.Slice || kind == reflect.Array {
		rules, err := compileRules(tag, typ.Elem())
		if err != nil {
			return nil, err
		}
		var errs []error
		for i := 0; i < fv.Len(); i++ {
			for _, r := range rules {
				if e := r.Validate(fv.Index(i)); e != nil {
					errs = append(errs, fmt.Errorf("element %d: %w", i, e))
				}
			}
		}
		return errs, nil
	}

	rules, err := compileRules(tag, typ)
	if err != nil {
		return nil, err
	}
	var errs []error
	for _, r := range rules {
		if e := r.Validate(fv); e != nil {
			errs = append(errs, e)
		}
	}
	return errs, nil
}

func validateNested(prefix string, fv reflect.Value) (ValidationErrors, error) {
	kind := fv.Kind()

	if kind == reflect.Ptr || kind == reflect.Interface {
		if fv.IsNil() {
			return nil, nil
		}
		return validateNested(prefix, fv.Elem())
	}

	if kind == reflect.Struct {
		err := Validate(fv.Interface())
		if err == nil {
			return nil, nil
		}
		var ve ValidationErrors
		if !errors.As(err, &ve) {
			return nil, err
		}
		out := make(ValidationErrors, 0, len(ve))
		for _, e := range ve {
			out = append(out, ValidationError{Field: prefix + "." + e.Field, Err: e.Err})
		}
		return out, nil
	}

	if kind == reflect.Slice || kind == reflect.Array {
		var out ValidationErrors
		for i := 0; i < fv.Len(); i++ {
			nested, err := validateNested(fmt.Sprintf("%s[%d]", prefix, i), fv.Index(i))
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
		}
		return out, nil
	}

	return nil, fmt.Errorf("%w: %q cannot be validated as nested (%s)",
		ErrUnsupportedType, prefix, kind)
}

type validator interface {
	Validate(v reflect.Value) error
}

var intKinds = map[reflect.Kind]bool{
	reflect.Int: true, reflect.Int8: true, reflect.Int16: true,
	reflect.Int32: true, reflect.Int64: true,
	reflect.Uint: true, reflect.Uint8: true, reflect.Uint16: true,
	reflect.Uint32: true, reflect.Uint64: true,
}

func compileRules(tag string, typ reflect.Type) ([]validator, error) {
	parts := strings.Split(tag, "|")
	rules := make([]validator, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, param, found := strings.Cut(part, ":")
		if !found {
			return nil, fmt.Errorf("%w: %q (expected name:param)", ErrInvalidRule, part)
		}
		r, err := compileRule(name, param, typ)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func compileRule(name, param string, typ reflect.Type) (validator, error) {
	kind := typ.Kind()
	switch {
	case kind == reflect.String:
		return compileStringRule(name, param)
	case intKinds[kind]:
		return compileIntRule(name, param)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, kind)
	}
}

func compileStringRule(name, param string) (validator, error) {
	switch name {
	case "len":
		n, err := strconv.Atoi(param)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%w: len:%q must be a non-negative integer", ErrInvalidRuleParam, param)
		}
		return lenRule{n: n}, nil
	case "regexp":
		re, err := compileRegexp(param)
		if err != nil {
			return nil, fmt.Errorf("%w: regexp:%q: %w", ErrInvalidRuleParam, param, err)
		}
		return regexpRule{re: re}, nil
	case "in":
		return inStringRule{set: strings.Split(param, ",")}, nil
	default:
		return nil, fmt.Errorf("%w: %q for string", ErrUnknownRule, name)
	}
}

type lenRule struct{ n int }

func (r lenRule) Validate(v reflect.Value) error {
	if got := utf8.RuneCountInString(v.String()); got != r.n {
		return fmt.Errorf("%w: expected %d characters, got %d", ErrLen, r.n, got)
	}
	return nil
}

type regexpRule struct{ re *regexp.Regexp }

func (r regexpRule) Validate(v reflect.Value) error {
	if !r.re.MatchString(v.String()) {
		return fmt.Errorf("%w: %q must match %q", ErrRegexp, v.String(), r.re.String())
	}
	return nil
}

type inStringRule struct{ set []string }

func (r inStringRule) Validate(v reflect.Value) error {
	s := v.String()
	for _, allowed := range r.set {
		if s == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: %q must be one of [%s]", ErrIn, s, strings.Join(r.set, ", "))
}

func compileIntRule(name, param string) (validator, error) {
	switch name {
	case "min":
		n, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: min:%q must be an integer", ErrInvalidRuleParam, param)
		}
		return minRule{limit: n}, nil
	case "max":
		n, err := strconv.ParseInt(param, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: max:%q must be an integer", ErrInvalidRuleParam, param)
		}
		return maxRule{limit: n}, nil
	case "in":
		raw := strings.Split(param, ",")
		set := make([]int64, 0, len(raw))
		for _, s := range raw {
			n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: in:%q must be a list of integers", ErrInvalidRuleParam, param)
			}
			set = append(set, n)
		}
		return inIntRule{set: set}, nil
	default:
		return nil, fmt.Errorf("%w: %q for integer", ErrUnknownRule, name)
	}
}

func asInt64(v reflect.Value) (int64, bool) {
	if v.CanUint() {
		u := v.Uint()
		if u > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(u), true
	}
	return v.Int(), true
}

func numString(v reflect.Value) string {
	if v.CanUint() {
		return strconv.FormatUint(v.Uint(), 10)
	}
	return strconv.FormatInt(v.Int(), 10)
}

type minRule struct{ limit int64 }

func (r minRule) Validate(v reflect.Value) error {
	n, ok := asInt64(v)
	if !ok {
		return nil // значение заведомо больше любого int64-порога
	}
	if n < r.limit {
		return fmt.Errorf("%w: %d is less than %d", ErrMin, n, r.limit)
	}
	return nil
}

type maxRule struct{ limit int64 }

func (r maxRule) Validate(v reflect.Value) error {
	n, ok := asInt64(v)
	if !ok || n > r.limit {
		return fmt.Errorf("%w: %s is greater than %d", ErrMax, numString(v), r.limit)
	}
	return nil
}

type inIntRule struct{ set []int64 }

func (r inIntRule) Validate(v reflect.Value) error {
	if n, ok := asInt64(v); ok {
		for _, allowed := range r.set {
			if n == allowed {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: %s must be one of %v", ErrIn, numString(v), r.set)
}

var reCache sync.Map

func compileRegexp(pattern string) (*regexp.Regexp, error) {
	if cached, ok := reCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	reCache.Store(pattern, re)
	return re, nil
}
