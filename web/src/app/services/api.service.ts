import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class ApiService {
  private apiUrl = '/api';

  constructor(private http: HttpClient) { }

  /**
   * Generic GET request
   */
  get<T>(endpoint: string, params?: any, headers?: HttpHeaders): Observable<T> {
    const options = {
      params: new HttpParams({ fromObject: params || {} }),
      headers
    };
    return this.http.get<T>(`${this.apiUrl}${endpoint}`, options);
  }

  /**
   * Generic POST request
   */
  post<T>(endpoint: string, body: any, headers?: HttpHeaders): Observable<T> {
    return this.http.post<T>(`${this.apiUrl}${endpoint}`, body, { headers });
  }

  /**
   * Generic PUT request
   */
  put<T>(endpoint: string, body: any, headers?: HttpHeaders): Observable<T> {
    return this.http.put<T>(`${this.apiUrl}${endpoint}`, body, { headers });
  }

  /**
   * Generic PATCH request
   */
  patch<T>(endpoint: string, body: any, headers?: HttpHeaders): Observable<T> {
    return this.http.patch<T>(`${this.apiUrl}${endpoint}`, body, { headers });
  }

  /**
   * Generic DELETE request
   */
  delete<T>(endpoint: string, headers?: HttpHeaders): Observable<T> {
    return this.http.delete<T>(`${this.apiUrl}${endpoint}`, { headers });
  }
/**
   * Placeholder for creating a bookmark
   * TODO: Update endpoint and request/response types when API is defined
   */
  createBookmark(bookmarkData?: any): Observable<any> {
    // Assuming a POST request to a '/bookmarks' endpoint
    return this.post<any>('/auth/register', bookmarkData || {});
  }

  /**
   * Fetches a new UUID from the backend.
   */
  fetchUuid(): Observable<{ uuid: string }> {
    return this.post<{ uuid: string }>('/v1/utils/uuid', {});
  }
}