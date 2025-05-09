import { Injectable, signal } from '@angular/core';
import { Title } from '@angular/platform-browser';

@Injectable({
  providedIn: 'root'
})
export class TitleService {
  // Using signals for reactive state management
  private titleSignal = signal<string>('Shiori');
  
  constructor(private documentTitle: Title) {
    // Set the initial document title
    this.updateDocumentTitle(this.getTitle());
  }
  
  getTitle(): string {
    return this.titleSignal();
  }
  
  setTitle(newTitle: string): void {
    // console.log('Title changed from', this.titleSignal(), 'to', newTitle);
    this.titleSignal.set(newTitle);
    
    // Update the document title whenever the title changes
    this.updateDocumentTitle(newTitle);
  }
  
  private updateDocumentTitle(title: string): void {
    // Update the browser tab title
    this.documentTitle.setTitle(title);
  }
}