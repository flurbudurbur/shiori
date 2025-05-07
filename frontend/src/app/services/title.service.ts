import { Injectable, signal } from '@angular/core';

@Injectable({
  providedIn: 'root'
})
export class TitleService {
  // Using signals for reactive state management
  private titleSignal = signal<string>('SyncYomi');
  
  constructor() {
    // console.log('TitleService initialized with title:', this.getTitle());
  }
  
  getTitle(): string {
    return this.titleSignal();
  }
  
  setTitle(newTitle: string): void {
    // console.log('Title changed from', this.titleSignal(), 'to', newTitle);
    this.titleSignal.set(newTitle);
  }
}