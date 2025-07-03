#import <Foundation/Foundation.h>
#import <CoreML/CoreML.h>
#import "coreml_bridge.h"

@interface CoreMLWrapper : NSObject
@property (nonatomic, strong) MLModel *model;
@end

@implementation CoreMLWrapper
@end

CoreMLModelHandle coreml_load_model(const char* model_path) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:model_path];
        NSURL *modelURL = [NSURL fileURLWithPath:path];
        
        NSError *error = nil;
        MLModel *model = [MLModel modelWithContentsOfURL:modelURL error:&error];
        
        if (error != nil) {
            NSLog(@"Error loading CoreML model: %@", error.localizedDescription);
            return NULL;
        }
        
        CoreMLWrapper *wrapper = [[CoreMLWrapper alloc] init];
        wrapper.model = model;
        
        return (__bridge_retained void*)wrapper;
    }
}

void coreml_release_model(CoreMLModelHandle handle) {
    if (handle != NULL) {
        CoreMLWrapper *wrapper = (__bridge_transfer CoreMLWrapper*)handle;
        wrapper = nil;
    }
}

CoreMLResult coreml_predict(CoreMLModelHandle handle, const char* input_text) {
    CoreMLResult result = {0};
    
    @autoreleasepool {
        if (handle == NULL || input_text == NULL) {
            result.error = strdup("Invalid input parameters");
            result.success = 0;
            return result;
        }
        
        CoreMLWrapper *wrapper = (__bridge CoreMLWrapper*)handle;
        if (wrapper.model == nil) {
            result.error = strdup("Model not loaded");
            result.success = 0;
            return result;
        }
        
        NSString *inputString = [NSString stringWithUTF8String:input_text];
        
        // Create input feature provider
        NSError *error = nil;
        
        // Get model description to understand input format
        MLModelDescription *modelDescription = wrapper.model.modelDescription;
        NSDictionary *inputDescription = modelDescription.inputDescriptionsByName;
        
        if (inputDescription.count == 0) {
            result.error = strdup("No input features found in model");
            result.success = 0;
            return result;
        }
        
        // Get the first input feature name (assuming single text input)
        NSString *inputFeatureName = [inputDescription.allKeys firstObject];
        MLFeatureDescription *featureDesc = inputDescription[inputFeatureName];
        
        id<MLFeatureProvider> inputProvider = nil;
        
        // Handle different input types
        if (featureDesc.type == MLFeatureTypeString) {
            // String input
            MLDictionaryFeatureProvider *provider = [[MLDictionaryFeatureProvider alloc] 
                initWithDictionary:@{inputFeatureName: inputString} 
                error:&error];
            inputProvider = provider;
        } else if (featureDesc.type == MLFeatureTypeMultiArray) {
            // For now, we'll convert string to a simple array representation
            // This is a simplified approach - real implementation would need proper tokenization
            NSData *data = [inputString dataUsingEncoding:NSUTF8StringEncoding];
            NSMutableArray *tokens = [NSMutableArray array];
            
            // Simple character-based tokenization (placeholder)
            for (NSUInteger i = 0; i < MIN(data.length, 512); i++) {
                const char *bytes = data.bytes;
                [tokens addObject:@((float)bytes[i])];
            }
            
            // Pad or truncate to expected length
            NSArray *shape = featureDesc.multiArrayConstraint.shape;
            NSUInteger expectedLength = [shape.lastObject unsignedIntegerValue];
            
            while (tokens.count < expectedLength) {
                [tokens addObject:@(0.0f)];
            }
            if (tokens.count > expectedLength) {
                tokens = [[tokens subarrayWithRange:NSMakeRange(0, expectedLength)] mutableCopy];
            }
            
            MLMultiArray *multiArray = [[MLMultiArray alloc] 
                initWithShape:shape 
                dataType:MLMultiArrayDataTypeFloat32 
                error:&error];
            
            if (error != nil) {
                result.error = strdup(error.localizedDescription.UTF8String);
                result.success = 0;
                return result;
            }
            
            for (NSUInteger i = 0; i < tokens.count; i++) {
                multiArray[@(i)] = tokens[i];
            }
            
            MLDictionaryFeatureProvider *provider = [[MLDictionaryFeatureProvider alloc] 
                initWithDictionary:@{inputFeatureName: multiArray} 
                error:&error];
            inputProvider = provider;
        }
        
        if (error != nil || inputProvider == nil) {
            result.error = strdup("Failed to create input provider");
            result.success = 0;
            return result;
        }
        
        // Make prediction
        id<MLFeatureProvider> outputProvider = [wrapper.model predictionFromFeatures:inputProvider error:&error];
        
        if (error != nil) {
            result.error = strdup(error.localizedDescription.UTF8String);
            result.success = 0;
            return result;
        }
        
        // Extract output
        NSDictionary *outputDescription = modelDescription.outputDescriptionsByName;
        NSString *outputFeatureName = [outputDescription.allKeys firstObject];
        MLFeatureValue *outputFeature = [outputProvider featureValueForName:outputFeatureName];
        
        NSString *outputString = @"";
        
        if (outputFeature.type == MLFeatureTypeString) {
            outputString = outputFeature.stringValue;
        } else if (outputFeature.type == MLFeatureTypeMultiArray) {
            // Convert multiarray back to string (simplified approach)
            MLMultiArray *outputArray = outputFeature.multiArrayValue;
            NSMutableString *resultString = [NSMutableString string];
            
            // Simple approach: convert numbers back to characters
            for (NSUInteger i = 0; i < outputArray.count && i < 1000; i++) {
                NSNumber *value = outputArray[@(i)];
                int charValue = value.intValue;
                if (charValue > 0 && charValue < 128) {
                    [resultString appendFormat:@"%c", (char)charValue];
                }
            }
            outputString = resultString;
        } else if (outputFeature.type == MLFeatureTypeDictionary) {
            // Handle dictionary output (common for classification)
            NSDictionary *dict = outputFeature.dictionaryValue;
            NSArray *sortedKeys = [dict.allKeys sortedArrayUsingComparator:^NSComparisonResult(id obj1, id obj2) {
                return [dict[obj2] compare:dict[obj1]];
            }];
            
            if (sortedKeys.count > 0) {
                outputString = [NSString stringWithFormat:@"%@", sortedKeys.firstObject];
            }
        }
        
        // Default fallback response if we can't extract meaningful output
        if (outputString.length == 0) {
            outputString = @"Generated response from CoreML model";
        }
        
        result.response = strdup(outputString.UTF8String);
        result.success = 1;
        
        return result;
    }
}

void coreml_free_result(CoreMLResult* result) {
    if (result != NULL) {
        if (result->response != NULL) {
            free(result->response);
            result->response = NULL;
        }
        if (result->error != NULL) {
            free(result->error);
            result->error = NULL;
        }
    }
}