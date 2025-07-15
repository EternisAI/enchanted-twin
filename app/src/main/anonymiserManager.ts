import path from 'node:path'
import fs from 'node:fs'
import { spawn } from 'node:child_process'
import log from 'electron-log/main'

import { PythonEnvironmentManager } from './pythonEnvironmentManager'
import {
  AnonymiserCallbacks,
  AnonymiserResult,
  AnonymiserConfig,
  DependencyProgress
} from './types/pythonManager.types'

const ANONYMISER_PROGRESS_STEPS = {
  VENV_SETUP: 20,
  PROJECT_FILES: 40,
  DEPENDENCIES: 70,
  MODEL_INITIALIZATION: 90,
  COMPLETE: 100
} as const

export class AnonymiserManager {
  private readonly projectName = 'anonymiser'
  private readonly pythonEnv: PythonEnvironmentManager
  private readonly projectDir: string
  private readonly modelFile: string
  private readonly configFile: string

  private onProgress?: (data: DependencyProgress) => void
  private onModelReady?: (ready: boolean) => void
  private onProcessingComplete?: (result: AnonymiserResult) => void
  private anonymiserConfig: AnonymiserConfig
  private childProcess: import('child_process').ChildProcess | null = null
  private latestProgress: DependencyProgress

  constructor(config: AnonymiserConfig = {}, callbacks: AnonymiserCallbacks = {}) {
    this.pythonEnv = new PythonEnvironmentManager()
    this.projectDir = this.pythonEnv.getProjectDir(this.projectName)
    this.modelFile = path.join(this.projectDir, 'anonymiser.py')
    this.configFile = path.join(this.projectDir, 'config.json')

    this.onProgress = callbacks.onProgress
    this.anonymiserConfig = config
    this.onModelReady = callbacks.onModelReady
    this.onProcessingComplete = callbacks.onProcessingComplete

    this.latestProgress = {
      dependency: 'Anonymiser',
      progress: 0,
      status: 'Not started'
    }
  }

  getLatestProgress(): DependencyProgress {
    return this.latestProgress
  }

  private updateProgress(progress: number, status: string, error?: string) {
    const progressData: DependencyProgress = {
      dependency: 'Anonymiser',
      progress,
      status,
      error
    }
    this.latestProgress = progressData
    this.onProgress?.(progressData)
  }

  private async setupProjectFiles(): Promise<void> {
    log.info('[Anonymiser] Setting up anonymiser files')
    await this.pythonEnv.ensureProjectDirectory(this.projectName)

    // Create mock anonymiser Python script
    const anonymiserCode = `#!/usr/bin/env python3
"""
Mock Anonymiser Implementation
This is a placeholder implementation for the anonymiser service.
"""
import json
import sys
import time
import logging
from typing import Dict, List, Any

# Set up logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class MockAnonymiser:
    def __init__(self, config_path: str = "config.json"):
        self.config = self.load_config(config_path)
        self.model_ready = False
        logger.info("Mock Anonymiser initialized")
    
    def load_config(self, config_path: str) -> Dict[str, Any]:
        try:
            with open(config_path, 'r') as f:
                return json.load(f)
        except FileNotFoundError:
            logger.warning(f"Config file {config_path} not found, using defaults")
            return {
                "model_name": "mock-anonymiser-v1",
                "confidence_threshold": 0.8,
                "enabled_entity_types": ["PERSON", "EMAIL", "PHONE_NUMBER", "SSN", "CREDIT_CARD"]
            }
    
    def initialize_model(self):
        """Mock model initialization"""
        logger.info("Initializing anonymiser model...")
        # Simulate model loading time
        time.sleep(2)
        self.model_ready = True
        logger.info("MODEL_READY: true")
        print("STATE:{\\"state\\": \\"ready\\", \\"timestamp\\": " + str(int(time.time() * 1000)) + "}")
        sys.stdout.flush()
    
    def anonymise_text(self, text: str) -> Dict[str, Any]:
        """Mock text anonymisation"""
        if not self.model_ready:
            raise RuntimeError("Model not initialized")
        
        # Mock entity detection and replacement
        entities = []
        anonymised_text = text
        
        # Simple mock replacements
        if "john.doe@example.com" in text.lower():
            entities.append({
                "type": "EMAIL",
                "original": "john.doe@example.com",
                "replacement": "[EMAIL_REDACTED]",
                "start": text.lower().find("john.doe@example.com"),
                "end": text.lower().find("john.doe@example.com") + len("john.doe@example.com")
            })
            anonymised_text = anonymised_text.replace("john.doe@example.com", "[EMAIL_REDACTED]")
        
        if "john" in text.lower():
            start_idx = text.lower().find("john")
            entities.append({
                "type": "PERSON",
                "original": "John",
                "replacement": "[PERSON_1]",
                "start": start_idx,
                "end": start_idx + 4
            })
            anonymised_text = anonymised_text.replace("John", "[PERSON_1]").replace("john", "[PERSON_1]")
        
        return {
            "original_text": text,
            "anonymised_text": anonymised_text,
            "entities": entities
        }
    
    def process_input(self, input_data: Dict[str, Any]) -> Dict[str, Any]:
        """Process input and return anonymised result"""
        try:
            if input_data.get("action") == "anonymise":
                text = input_data.get("text", "")
                result = self.anonymise_text(text)
                logger.info(f"Anonymised text: {len(text)} chars -> {len(result['anonymised_text'])} chars")
                return {
                    "status": "success",
                    "result": result
                }
            else:
                return {
                    "status": "error",
                    "message": f"Unknown action: {input_data.get('action')}"
                }
        except Exception as e:
            logger.error(f"Error processing input: {e}")
            return {
                "status": "error",
                "message": str(e)
            }

def main():
    anonymiser = MockAnonymiser()
    anonymiser.initialize_model()
    
    logger.info("Anonymiser service ready, waiting for input...")
    
    try:
        while True:
            line = sys.stdin.readline()
            if not line:
                break
            
            try:
                input_data = json.loads(line.strip())
                result = anonymiser.process_input(input_data)
                print(json.dumps(result))
                sys.stdout.flush()
            except json.JSONDecodeError as e:
                logger.error(f"Invalid JSON input: {e}")
                print(json.dumps({
                    "status": "error",
                    "message": f"Invalid JSON: {e}"
                }))
                sys.stdout.flush()
    except KeyboardInterrupt:
        logger.info("Anonymiser service stopped")
    except Exception as e:
        logger.error(f"Unexpected error: {e}")

if __name__ == "__main__":
    main()
`

    await fs.promises.writeFile(this.modelFile, anonymiserCode)

    // Create configuration file
    const configData = {
      model_name: this.anonymiserConfig.modelName || 'mock-anonymiser-v1',
      confidence_threshold: this.anonymiserConfig.confidenceThreshold || 0.8,
      enabled_entity_types: this.anonymiserConfig.enabledEntityTypes || [
        'PERSON',
        'EMAIL',
        'PHONE_NUMBER',
        'SSN',
        'CREDIT_CARD'
      ]
    }

    await fs.promises.writeFile(this.configFile, JSON.stringify(configData, null, 2))
    log.info('[Anonymiser] Anonymiser files created successfully')
  }

  private getAdditionalEnvironmentVariables(): Record<string, string> {
    return {
      PYTHONUNBUFFERED: '1',
      NO_COLOR: '1',
      ANONYMISER_CONFIG: this.configFile
    }
  }

  async setup(): Promise<void> {
    log.info('[Anonymiser] Starting Anonymiser setup process')
    try {
      this.updateProgress(ANONYMISER_PROGRESS_STEPS.VENV_SETUP, 'Setting up virtual environment')
      await this.pythonEnv.setupProjectVenv(this.projectName)

      this.updateProgress(ANONYMISER_PROGRESS_STEPS.PROJECT_FILES, 'Setting up project files')
      await this.setupProjectFiles()

      this.updateProgress(ANONYMISER_PROGRESS_STEPS.DEPENDENCIES, 'Installing dependencies')
      const dependencies = [
        'transformers>=4.30.0',
        'torch>=2.0.0',
        'spacy>=3.6.0',
        'presidio-analyzer>=2.2.0',
        'presidio-anonymizer>=2.2.0',
        'datasets>=2.14.0',
        'accelerate>=0.21.0'
      ]
      await this.pythonEnv.installDependencies(this.projectName, dependencies)

      this.updateProgress(ANONYMISER_PROGRESS_STEPS.MODEL_INITIALIZATION, 'Initializing model')
      await this.startAnonymiser()

      this.updateProgress(ANONYMISER_PROGRESS_STEPS.COMPLETE, 'Ready')
      log.info('[Anonymiser] Anonymiser setup completed successfully')
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[Anonymiser] Anonymiser setup failed', e)
      this.updateProgress(this.latestProgress.progress, 'Failed', error)

      throw new Error(`Anonymiser setup failed: ${error}`)
    }
  }

  async startAnonymiser(): Promise<void> {
    if (this.childProcess) {
      log.warn('[Anonymiser] Anonymiser is already running')
      return
    }

    log.info('[Anonymiser] Starting anonymiser service')

    this.childProcess = spawn(this.pythonEnv.getPythonBin(this.projectName), ['anonymiser.py'], {
      cwd: this.projectDir,
      env: {
        ...process.env,
        ...this.pythonEnv.getUvEnv(this.projectName),
        ...this.getAdditionalEnvironmentVariables()
      },
      stdio: 'pipe'
    })

    this.childProcess.stdout?.on('data', (data) => {
      this.handleAnonymiserOutput(data.toString())
    })

    this.childProcess.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[Anonymiser] ${output}`)
      }
    })

    this.childProcess.on('exit', (code) => {
      log.info(`[Anonymiser] Service exited with code ${code}`)
      this.onModelReady?.(false)
      this.childProcess = null
    })

    log.info('[Anonymiser] Anonymiser service started successfully')
  }

  private handleAnonymiserOutput(data: string): void {
    const lines = data.toString().trim().split('\n')
    for (const line of lines) {
      if (line.includes('MODEL_READY: true')) {
        this.onModelReady?.(true)
      } else if (line.startsWith('STATE:')) {
        try {
          const stateData = JSON.parse(line.substring(6))
          if (stateData.state === 'ready') {
            this.onModelReady?.(true)
          }
        } catch (error) {
          log.error('[Anonymiser] Failed to parse state update:', error)
        }
      } else if (line.startsWith('{') && line.endsWith('}')) {
        // Handle JSON result
        try {
          const result = JSON.parse(line)
          if (result.status === 'success' && result.result) {
            this.onProcessingComplete?.(result.result)
          }
        } catch (error) {
          log.error('[Anonymiser] Failed to parse result:', error)
        }
      } else if (line.trim()) {
        log.info(`[Anonymiser] ${line}`)
      }
    }
  }

  async anonymiseText(text: string): Promise<AnonymiserResult | null> {
    if (!this.childProcess || !this.childProcess.stdin) {
      log.warn('[Anonymiser] Cannot anonymise text: service not running')
      return null
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Anonymisation timeout'))
      }, 30000) // 30 second timeout

      const handleResult = (result: AnonymiserResult) => {
        clearTimeout(timeout)
        this.onProcessingComplete = undefined // Remove temporary handler
        resolve(result)
      }

      // Set temporary handler for this request
      this.onProcessingComplete = handleResult

      try {
        const request = {
          action: 'anonymise',
          text: text,
          timestamp: Date.now()
        }

        const requestStr = JSON.stringify(request) + '\n'
        this.childProcess!.stdin!.write(requestStr)
        log.info('[Anonymiser] Sent anonymisation request')
      } catch (error) {
        clearTimeout(timeout)
        this.onProcessingComplete = undefined
        reject(error)
      }
    })
  }

  async stopAnonymiser(): Promise<void> {
    await this.stopChildProcess()
    this.onModelReady?.(false)
  }

  private async stopChildProcess(): Promise<void> {
    if (!this.childProcess) {
      log.warn('[Anonymiser] No child process to stop')
      return
    }

    log.info('[Anonymiser] Stopping child process')
    this.childProcess.kill('SIGTERM')

    // Give it a moment to exit gracefully
    await new Promise((resolve) => setTimeout(resolve, 300))

    if (this.childProcess) {
      log.info('[Anonymiser] Force killing child process')
      this.childProcess.kill('SIGKILL')
    }

    this.childProcess = null
    log.info('[Anonymiser] Child process stopped')
  }

  isAnonymiserRunning(): boolean {
    return this.childProcess !== null
  }

  async cleanup(): Promise<void> {
    await this.stopAnonymiser()
  }
}
