import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "10s", target: 5 },
    { duration: "10s", target: 30 }, // Sudden honest burst
    { duration: "10s", target: 5 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.02"], // False positive test: honest bursts should NOT fail
  },
};

export default function () {
  const params = {
    headers: {
      "X-Client-ID": `client-burst-${__VU}`,
      "Content-Type": "application/json",
    },
  };
  const res = http.get("http://localhost:8080/api/users", params);
  check(res, {
    "not blocked": (r) => r.status === 200,
  });
  sleep(0.1);
}
