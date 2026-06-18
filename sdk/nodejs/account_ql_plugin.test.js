const test = require('node:test');
const assert = require('node:assert/strict');
const { builtinPaymentAuth } = require('./account_ql_plugin');

test('builtinPaymentAuth keeps point price config and payment options', () => {
  const provider = builtinPaymentAuth({
    priceConfig: 'auth_price_per_month',
    pointsPerRMBConfig: 'auth_payment_points_per_rmb',
    timeoutConfig: 'auth_payment_timeout',
    methodsConfig: 'auth_payment_methods',
    methods: ['alipay']
  });
  assert.equal(provider.type, 'builtin_payment');
  assert.equal(provider.priceConfig, 'auth_price_per_month');
  assert.equal(provider.pointsPerRMBConfig, 'auth_payment_points_per_rmb');
  assert.equal(provider.timeoutConfig, 'auth_payment_timeout');
  assert.equal(provider.methodsConfig, 'auth_payment_methods');
  assert.deepEqual(provider.methods, ['alipay']);
});
