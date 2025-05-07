import { Injectable } from '@angular/core';
import { ApiService } from './api.service';
import { Observable, catchError, of, tap } from 'rxjs';

export interface ConfigJson {
  // Define the configuration interface based on the OpenAPI schema
  // This is a simplified version, expand as needed
  baseUrl?: string;
  logLevel?: string;
  port?: number;
  enableAuth?: boolean;
  databaseType?: string;
  databasePath?: string;
  // Add other configuration properties as needed
}

export interface ConfigUpdate {
  // Define the update configuration interface
  // This is a simplified version, expand as needed
  baseUrl?: string;
  logLevel?: string;
  port?: number;
  enableAuth?: boolean;
  databaseType?: string;
  databasePath?: string;
  // Add other configuration properties as needed
}

@Injectable({
  providedIn: 'root'
})
export class ConfigService {
  private config: ConfigJson | null = null;

  constructor(private apiService: ApiService) { }

  /**
   * Get server configuration
   */
  getConfig(): Observable<ConfigJson> {
    return this.apiService.get<ConfigJson>('/config').pipe(
      tap(config => {
        this.config = config;
      }),
      catchError(error => {
        console.error('Failed to get config:', error);
        return of({} as ConfigJson);
      })
    );
  }

  /**
   * Update server configuration
   */
  updateConfig(configUpdate: ConfigUpdate): Observable<any> {
    return this.apiService.patch<any>('/config', configUpdate).pipe(
      tap(() => {
        // Update local config with the changes
        this.config = { ...this.config, ...configUpdate };
      }),
      catchError(error => {
        console.error('Failed to update config:', error);
        return of({ error: 'Failed to update configuration' });
      })
    );
  }

  /**
   * Get cached configuration or fetch from server if not available
   */
  getCachedConfig(): Observable<ConfigJson> {
    if (this.config) {
      return of(this.config);
    }
    return this.getConfig();
  }
}