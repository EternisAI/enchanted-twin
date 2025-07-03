#ifndef COREML_BRIDGE_H
#define COREML_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef void* CoreMLModelHandle;

// Model management
CoreMLModelHandle coreml_load_model(const char* model_path);
void coreml_release_model(CoreMLModelHandle handle);

// Inference
typedef struct {
    char* response;
    char* error;
    int success;
} CoreMLResult;

CoreMLResult coreml_predict(CoreMLModelHandle handle, const char* input_text);
void coreml_free_result(CoreMLResult* result);

#ifdef __cplusplus
}
#endif

#endif // COREML_BRIDGE_H