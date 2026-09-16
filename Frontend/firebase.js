import { initializeApp } from "https://www.gstatic.com/firebasejs/10.12.0/firebase-app.js";
import { getMessaging, getToken, onMessage } 
from "https://www.gstatic.com/firebasejs/10.12.0/firebase-messaging.js";

const firebaseConfig = {
  apiKey: "AIzaSyBPL1fY-gLsgl5HzRAtpMfmi-XJR7Mh7XY",
  authDomain: "fir-b9eea.firebaseapp.com",
  projectId: "fir-b9eea",
  storageBucket: "fir-b9eea.firebasestorage.app",
  messagingSenderId: "854375414235",
  appId: "1:854375414235:web:0833acba8e2de2f6a54ffe"
};

const app = initializeApp(firebaseConfig);
const messaging = getMessaging(app);

navigator.serviceWorker.register("/firebase-messaging-sw.js");


async function initFCM() {
  const permission = await Notification.requestPermission();

  if (permission === "granted") {
    const token = await getToken(messaging, {
      vapidKey: "BJ69xGibgyZ6UnXuDTNLSVZPlL08foCrLSzbXjrTyjEHdCU5xNfI_5Q10pAPtHpJxOKhI9UJ5md63C32Mx2yA08"
    });

    console.log("Token:", token);

    // send token to backend
    await fetch("/save-token", {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify({ token })
    });
  }
}

initFCM();


onMessage(messaging, (payload) => {
  console.log("Message received:", payload);
  alert(payload.notification.title + " - " + payload.notification.body);
});

function same() {
  console.log("Same");
}

async function sendNotification() { 
    await fetch("/send"); 
};



window.sendNotification = sendNotification; 

window.same = same;