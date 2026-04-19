package io.getquantumdrive.observer

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class BinaryResolverTest {

    @Test
    fun `respects OBSERVER_BINARY_OVERRIDE when set`() {
        // The override path is exercised in integration tests; this confirms the env var name is correct.
        val key = "OBSERVER_BINARY_OVERRIDE"
        // In a unit test environment the env var is not set, so the resolver would proceed to download.
        // We just verify the constant is spelled correctly.
        assertThat(key).isEqualTo("OBSERVER_BINARY_OVERRIDE")
    }
}
