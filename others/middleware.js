fetch("https://api.fish.xzgkj.top/v1/beans/user-clockin", {
  "headers": {
    "accept": "*/*",
    "accept-language": "zh-CN,zh;q=0.9",
    "authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJtZGRhcGkiLCJleHAiOjE3ODAyNDYyNzAsImlhdCI6MTc3ODk1MDI3MCwibmJmIjoxNzc4OTUwMjcwLCJ1aWQiOjM4OTYxfQ.LHdAmqDeR9x2ppScacEwRmcDhHgj9hsyK5oiMTE5M1I",
    "content-type": "application/json"
  },
  "body": "{}",
  "method": "POST"
})
.then(res => res.text())
.then(text => {
    console.log("原始响应:", text);
    // 这里应该能看到加密的Base64字符串
})