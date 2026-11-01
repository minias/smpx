import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import Video from './Video.svelte'

// 영상 창(자식 프로세스)은 VideoApp 이 바인딩되어 있음
const isVideoWindow = Boolean(window.go?.main?.VideoApp)

// Svelte 5: 클래스 생성자(new App) 대신 mount 사용
const app = mount(isVideoWindow ? Video : App, {
  target: document.getElementById('app')
})

export default app
