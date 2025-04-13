package conflate

import (
    "fmt"
    "reflect"
)

// Config holds merge configuration options.
type Config struct {
    // ListsAsEntities, if true, replaces destination slices with source slices instead of appending.
    ListsAsEntities bool
}

// DefaultConfig is the default configuration.
var DefaultConfig = Config{
    ListsAsEntities: false, // Preserve original append behavior
}

// mergeTo merges multiple source data into the destination with the default config.
func mergeTo(toData interface{}, fromData ...interface{}) error {
    return mergeToWithConfig(toData, DefaultConfig, fromData...)
}

// mergeToWithConfig merges multiple source data into the destination with a custom config.
func mergeToWithConfig(toData interface{}, config Config, fromData ...interface{}) error {
    for _, fromDatum := range fromData {
        err := mergeWithConfig(toData, fromDatum, config)
        if err != nil {
            return err
        }
    }
    return nil
}

// merge merges a single source into the destination with the default config.
func merge(pToData, fromData interface{}) error {
    return mergeWithConfig(pToData, fromData, DefaultConfig)
}

// mergeWithConfig merges a single source into the destination with a custom config.
func mergeWithConfig(pToData, fromData interface{}, config Config) error {
    return mergeRecursive(rootContext(), pToData, fromData, config)
}

// mergeRecursive performs the recursive merge operation.
func mergeRecursive(ctx context, pToData, fromData interface{}, config Config) error {
    if pToData == nil {
        return &contextError{
            context: ctx,
            msg:     "the destination variable must not be nil",
        }
    }

    pToVal := reflect.ValueOf(pToData)
    if pToVal.Kind() != reflect.Ptr {
        return &contextError{
            context: ctx,
            msg:     "the destination variable must be a pointer",
        }
    }

    if fromData == nil {
        return nil
    }

    toVal := pToVal.Elem()
    fromVal := reflect.ValueOf(fromData)

    toData := toVal.Interface()

    if toVal.Interface() == nil {
        toVal.Set(fromVal)
        return nil
    }

    var err error

    switch fromVal.Kind() {
    case reflect.Map:
        err = mergeMapRecursive(ctx, toData, fromData)
    case reflect.Slice:
        err = mergeSliceRecursive(ctx, toVal, toData, fromData, config)
    default:
        err = mergeDefaultRecursive(ctx, toVal, fromVal, toData, fromData)
    }

    return err
}

// mergeMapRecursive merges map data.
func mergeMapRecursive(ctx context, toData, fromData interface{}) error {
    fromProps, ok := fromData.(map[string]interface{})
    if !ok {
        return &contextError{
            context: ctx,
            msg:     "the source value must be a map[string]interface{}",
        }
    }

    toProps, ok := toData.(map[string]interface{})
    if toProps == nil || !ok {
        return &contextError{
            context: ctx,
            msg:     "the destination value must be a map[string]interface{}",
        }
    }

    for name, fromProp := range fromProps {
        // Always set the source value, even if empty, unless it's nil
        if fromProp == nil {
            // Skip nil values to preserve existing destination values
            continue
        }
        // If the destination has a value, merge recursively; otherwise, set directly
        if val, exists := toProps[name]; exists && val != nil && fromProp != nil {
            err := merge(&val, fromProp)
            if err != nil {
                return &contextError{
                    context: ctx.add(name),
                    msg:     fmt.Sprintf("failed to merge object property : %v : %v", name, err.Error()),
                }
            }
            toProps[name] = val
        } else {
            toProps[name] = fromProp
        }
    }

    return nil
}

// mergeSliceRecursive merges slice data.
func mergeSliceRecursive(ctx context, toVal reflect.Value, toData, fromData interface{}, config Config) error {
    fromItems, ok := fromData.([]interface{})
    if !ok {
        return &contextError{
            context: ctx,
            msg:     "the source value must be a []interface{}",
        }
    }

    toItems, ok := toData.([]interface{})
    if toItems == nil || !ok {
        return &contextError{
            context: ctx,
            msg:     "the destination value must be a []interface{}",
        }
    }

    if config.ListsAsEntities {
        // Replace the destination slice with the source slice
        toVal.Set(reflect.ValueOf(fromItems))
    } else {
        // Append source items to destination (original behavior)
        toItems = append(toItems, fromItems...)
        toVal.Set(reflect.ValueOf(toItems))
    }

    return nil
}

// mergeDefaultRecursive handles non-map, non-slice data.
func mergeDefaultRecursive(ctx context, toVal, fromVal reflect.Value, toData, fromData interface{}) error {
    if reflect.DeepEqual(toData, fromData) {
        return nil
    }

    fromType := fromVal.Type()
    toType := toVal.Type()

    if toType.Kind() == reflect.Interface {
        toType = toVal.Elem().Type()
    }

    if !fromType.AssignableTo(toType) {
        return &contextError{
            context: ctx,
            msg:     fmt.Sprintf("the destination type (%v) must be the same as the source type (%v)", toType, fromType),
        }
    }

    toVal.Set(fromVal)
    return nil
}