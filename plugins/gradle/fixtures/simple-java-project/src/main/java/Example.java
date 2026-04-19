import java.security.KeyPairGenerator;
import javax.crypto.Cipher;

public class Example {
    public static void main(String[] args) throws Exception {
        // Deliberate RSA usage for scanner fixture testing
        KeyPairGenerator kpg = KeyPairGenerator.getInstance("RSA");
        kpg.initialize(2048);

        Cipher cipher = Cipher.getInstance("RSA/ECB/PKCS1Padding");
    }
}
