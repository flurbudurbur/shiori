import { Injectable } from '@angular/core';
import { ApiService } from './api.service';
import { Observable, catchError, of } from 'rxjs';
import { HttpHeaders } from '@angular/common/http'; // Import HttpHeaders

export interface Sync {
  // Define the sync interface based on the OpenAPI schema
  // This is a simplified version, expand as needed
  id?: number;
  apiKey?: string;
  data?: any; // Change type to any to accommodate potentially varied sync data
  deviceId?: string;
  timestamp?: string;
  // Add other sync properties as needed
}

@Injectable({
  providedIn: 'root'
})
export class SyncService {
  constructor(private apiService: ApiService) { }

  /**
   * Get sync content using API Key
   */
  getSyncContent(apiKey: string): Observable<Sync> {
    const headers = new HttpHeaders({
      'X-API-Token': apiKey
    });
    return this.apiService.get<Sync>('/sync/content', undefined, headers).pipe(
      catchError(error => {
        console.error('Failed to get sync content:', error);
        return of({} as Sync); // Return an empty Sync object on error
      })
    );
  }

  /**
   * Update sync content using API Key
   */
  updateSyncContent(apiKey: string, syncData: any): Observable<Sync> {
    const headers = new HttpHeaders({
      'X-API-Token': apiKey
    });
    return this.apiService.put<Sync>('/sync/content', syncData, headers).pipe(
      catchError(error => {
        console.error('Failed to update sync content:', error);
        return of({} as Sync); // Return an empty Sync object on error
      })
    );
  }
}