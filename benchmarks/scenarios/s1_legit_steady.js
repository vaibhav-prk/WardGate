import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 10,
  duration: "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<50"],
  },
};

export default function () {
  const params = {
    headers: {
      "X-Client-ID": `client-steady-${__VU}`,
      "Content-Type": "application/json",
    },
  };
  const res = http.get("http://localhost:8080/api/users", params);
  check(res, {
    "status is 200": (r) => r.status === 200,
  });
  sleep(0.5);
}
