package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrNotStruct                   = errors.New("wrong argument given, should be a struct")
	ErrInvalidValidatorSyntax      = errors.New("invalid validator syntax")
	ErrValidateForUnexportedFields = errors.New("validation for unexported field is not allowed")
	ErrLenValidationFailed         = errors.New("len validation failed")
	ErrInValidationFailed          = errors.New("in validation failed")
	ErrMaxValidationFailed         = errors.New("max validation failed")
	ErrMinValidationFailed         = errors.New("min validation failed")
)

type ValidationError struct {
	field string
	err   error
}

func NewValidationError(err error, field string) error {
	return &ValidationError{
		field: field,
		err:   err,
	}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.field, e.err)
}

func (e *ValidationError) Unwrap() error {
	return e.err
}

func checkLength(fieldName string, field reflect.Value, tag string) error {
	length, err := strconv.Atoi(tag)
	if err != nil {
		return NewValidationError(ErrInvalidValidatorSyntax, fieldName)
	}
	switch field.Kind() {
	case reflect.String:
		if len(field.String()) != length {
			return NewValidationError(ErrLenValidationFailed, fieldName)
		}
	case reflect.Slice:
		checkedStrings, ok := field.Interface().([]string)
		if !ok {
			return NewValidationError(errors.New("there are no strings in the slice"), fieldName)
		}
		for _, checkedString := range checkedStrings {
			if len(checkedString) != length {
				return NewValidationError(ErrLenValidationFailed, fieldName)
			}
		}
	default:
		return NewValidationError(errors.New("not supported type"), fieldName)
	}
	return nil
}

func checkIn(fieldName string, field reflect.Value, tag string) error {
	checkValues := make(map[string]struct{})
	for _, char := range strings.Split(tag, ",") {
		checkValues[char] = struct{}{}
	}
	value := ""
	if field.CanInt() {
		value = strconv.Itoa(int(field.Int()))
	} else {
		value = field.String()
	}
	if _, ok := checkValues[value]; ok {
		return nil
	}
	return NewValidationError(ErrInValidationFailed, fieldName)
}

func checkMin(fieldName string, field reflect.Value, tag string) error {
	checkValue, _ := strconv.Atoi(tag)
	switch field.Kind() {
	case reflect.Int, reflect.Int64:
		if int(field.Int()) < checkValue {
			return NewValidationError(ErrMinValidationFailed, fieldName)
		}
	case reflect.String:
		if len(field.String()) < checkValue {
			return NewValidationError(ErrMinValidationFailed, fieldName)
		}
	case reflect.Slice:
		switch field.Type().Elem().Kind() {
		case reflect.Int:
			for _, num := range field.Interface().([]int) {
				if num < checkValue {
					return NewValidationError(ErrMinValidationFailed, fieldName)
				}
			}
		case reflect.String:
			for _, str := range field.Interface().([]string) {
				if len(str) < checkValue {
					return NewValidationError(ErrMinValidationFailed, fieldName)
				}
			}
		default:
			return NewValidationError(errors.New("not supported type"), fieldName)
		}
	default:
		return NewValidationError(errors.New("not supported type"), fieldName)
	}
	return nil
}

func checkMax(fieldName string, field reflect.Value, tag string) error {
	checkValue, _ := strconv.Atoi(tag)
	switch field.Kind() {
	case reflect.Int:
		if int(field.Int()) > checkValue {
			return NewValidationError(ErrMaxValidationFailed, fieldName)
		}
	case reflect.String:
		if len(field.String()) > checkValue {
			return NewValidationError(ErrMaxValidationFailed, fieldName)
		}
	case reflect.Slice:
		switch field.Type().Elem().Kind() {
		case reflect.Int:
			for _, num := range field.Interface().([]int) {
				if num > checkValue {
					return NewValidationError(ErrMaxValidationFailed, fieldName)
				}
			}
		case reflect.String:
			for _, str := range field.Interface().([]string) {
				if len(str) > checkValue {
					return NewValidationError(ErrMaxValidationFailed, fieldName)
				}
			}
		default:
			return NewValidationError(errors.New("not supported type"), fieldName)
		}

	default:
		return NewValidationError(errors.New("not supported type"), fieldName)
	}
	return nil
}

func checkValidator(fieldName, tag string) (string, string, error) {
	split := strings.Split(tag, ":")
	validator := split[0]
	value := split[1]
	if validator == "len" || validator == "min" || validator == "max" {
		if _, err := strconv.Atoi(value); err != nil {
			return "", "", NewValidationError(ErrInvalidValidatorSyntax, fieldName)
		}
	}
	if validator == "len" {
		if v, _ := strconv.Atoi(value); v < 0 {
			return "", "", NewValidationError(ErrInvalidValidatorSyntax, fieldName)
		}
	}
	if validator == "in" {
		checkValues := strings.Split(value, ",")
		notEmptyCounter := 0
		for _, s := range checkValues {
			if s != "" {
				notEmptyCounter++
			}
		}
		if notEmptyCounter == 0 {
			return "", "", NewValidationError(ErrInvalidValidatorSyntax, fieldName)
		}
	}
	return validator, value, nil
}

func validateValue(reflectValue reflect.Value, resErrors *[]error) {
	valueType := reflectValue.Type()
	if reflectValue.Kind() != reflect.Struct {
		*resErrors = append(*resErrors, NewValidationError(ErrNotStruct, ""))
		return
	}
	for i := 0; i < reflectValue.NumField(); i++ {
		if reflectValue.Field(i).Kind() == reflect.Struct {
			validateValue(reflectValue.Field(i), resErrors)
			continue
		}
		if tag, ok := valueType.Field(i).Tag.Lookup("validate"); ok {
			if !valueType.Field(i).IsExported() {
				*resErrors = append(*resErrors, NewValidationError(ErrValidateForUnexportedFields, valueType.Field(i).Name))
				continue
			}
			var validator string
			var checkValue string
			var err error
			if validator, checkValue, err = checkValidator(valueType.Field(i).Name, tag); err != nil {
				*resErrors = append(*resErrors, err)
				continue
			}
			switch validator {
			case "len":
				if err := checkLength(valueType.Field(i).Name, reflectValue.Field(i), checkValue); err != nil {
					*resErrors = append(*resErrors, err)
				}
			case "in":
				if err := checkIn(valueType.Field(i).Name, reflectValue.Field(i), checkValue); err != nil {
					*resErrors = append(*resErrors, err)
				}
			case "min":
				if err := checkMin(valueType.Field(i).Name, reflectValue.Field(i), checkValue); err != nil {
					*resErrors = append(*resErrors, err)
				}
			case "max":
				if err := checkMax(valueType.Field(i).Name, reflectValue.Field(i), checkValue); err != nil {
					*resErrors = append(*resErrors, err)
				}
			}
		}

	}
}

func Validate(v any) error {
	resErrors := make([]error, 0)
	validateValue(reflect.ValueOf(v), &resErrors)
	return errors.Join(resErrors...)
}
