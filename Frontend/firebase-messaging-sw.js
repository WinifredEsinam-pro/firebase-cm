importScripts("https://www.gstatic.com/firebasejs/10.12.0/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/10.12.0/firebase-messaging-compat.js");

firebase.initializeApp({
  apiKey: "AIzaSyBPL1fY-gLsgl5HzRAtpMfmi-XJR7Mh7XY",
  authDomain: "fir-b9eea.firebaseapp.com",
  projectId: "fir-b9eea",
  messagingSenderId: "854375414235",
  appId: "1:854375414235:web:0833acba8e2de2f6a54ffe"
});

const messaging = firebase.messaging();