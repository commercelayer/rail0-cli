package cmd

import "testing"

func TestPaymentMethodsCommandTree(t *testing.T) {
	// `payment-methods` is top-level and lists when run bare (no positional args).
	sub, _, err := rootCmd.Find([]string{"payment-methods"})
	if err != nil || sub == nil || sub.Name() != "payment-methods" {
		t.Fatalf("payment-methods: not found (%v)", err)
	}
	if sub.RunE == nil {
		t.Error("payment-methods should list when run bare (nil RunE)")
	}
	if err := sub.Args(sub, []string{}); err != nil {
		t.Errorf("payment-methods should take no positional args, got %v", err)
	}
	if err := sub.Args(sub, []string{"x"}); err == nil {
		t.Error("payment-methods should reject positional args (use --account/--address)")
	}
	for _, f := range []string{"account", "address"} {
		if sub.Flags().Lookup(f) == nil {
			t.Errorf("payment-methods: --%s flag missing", f)
		}
	}
}

func TestPaymentMethodsRequiresExactlyOneHandle(t *testing.T) {
	sub, _, _ := rootCmd.Find([]string{"payment-methods"})

	// Neither handle → error (mirrors the gateway's exactly_one_of).
	paymentMethodsAccount, paymentMethodsAddress = "", ""
	if err := sub.RunE(sub, nil); err == nil {
		t.Error("expected an error when neither --account nor --address is given")
	}

	// Both handles → error.
	paymentMethodsAccount, paymentMethodsAddress = "acc_1", "0xabc"
	if err := sub.RunE(sub, nil); err == nil {
		t.Error("expected an error when both --account and --address are given")
	}

	// Reset the package-level flag state for other tests.
	paymentMethodsAccount, paymentMethodsAddress = "", ""
}
