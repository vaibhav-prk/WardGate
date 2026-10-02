import http from "k6/http";
import { check, sleep } from "k6";
import { generateHeaders } from "./auth_helper.js";

export const options = {
  stages: [
    { duration: "2s", target: 20 },
    { duration: "5s", target: 100 },
    { duration: "2s", target: 0 },
  ],
  thresholds: {
    checks: ["rate==1.0"],
  },
};

const BASE_URL = __ENV.TARGET_URL || "http://localhost:8080";

export default function () {
  const path = "/api/users";
  const headers = generateHeaders("GET", path, "");

  const res = http.get(`${BASE_URL}${path}`, {
    headers,
    responseCallback: http.expectedStatuses(200, 404, 429),
  });

  check(res, {
    "rate-limiting or upstream ok": (r) =>
      r.status === 200 || r.status === 404 || r.status === 429,
  });

  sleep(0.01);
}
