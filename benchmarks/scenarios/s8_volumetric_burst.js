import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    constant_request_rate: {
      executor: "constant-arrival-rate",
      rate: 500, // 500 req/sec overload
      timeUnit: "1s",
      duration: "20s",
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
};

export default function () {
  const params = {
    headers: {
      "X-Client-ID": "volumetric-attacker",
      "Content-Type": "application/json",
    },
  };
  const res = http.get("http://localhost:8080/api/users", params);
  check(res, {
    handled: (r) => r.status === 200 || r.status === 429,
  });
}
