#!/bin/bash
# Test all VTC files and report results

cd /home/user/GTest

echo "Testing all VTC files..."
echo "========================"

pass_count=0
fail_count=0
timeout_count=0

for test_file in tests/a*.vtc; do
    test_name=$(basename "$test_file")

    # Run test with 5 second timeout
    if timeout 5 ./gvtest "$test_file" > /dev/null 2>&1; then
        echo "✓ $test_name"
        ((pass_count++))
    else
        exit_code=$?
        if [ $exit_code -eq 124 ]; then
            echo "⏱ $test_name (TIMEOUT)"
            ((timeout_count++))
        else
            echo "✗ $test_name (EXIT: $exit_code)"
            ((fail_count++))
        fi
    fi
done

echo ""
echo "========================"
echo "Summary:"
echo "  Passed: $pass_count"
echo "  Failed: $fail_count"
echo "  Timeout: $timeout_count"
echo "  Total: $((pass_count + fail_count + timeout_count))"
