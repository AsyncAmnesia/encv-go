package com.encvgo.app;

import android.os.Bundle;
import android.util.Log;
import com.getcapacitor.BridgeActivity;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;

public class MainActivity extends BridgeActivity {
    private static final String TAG = "ENCV-go";
    private static final String BINARY_NAME = "encv-go";
    private Process goProcess = null;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        startGoDaemon();
    }

    private void startGoDaemon() {
        try {
            File binary = new File(getFilesDir(), BINARY_NAME);
            if (!binary.exists()) {
                copyBinaryFromAssets(binary);
            }
            binary.setExecutable(true);

            String configPath = new File(getFilesDir(), "config.user.json").getAbsolutePath();
            ProcessBuilder pb = new ProcessBuilder(binary.getAbsolutePath(), "start");
            pb.environment().put("ENCV_CONFIG", configPath);
            pb.redirectErrorStream(true);
            goProcess = pb.start();

            Log.i(TAG, "ENCV-go daemon started, PID: " + goProcess.pid());
        } catch (Exception e) {
            Log.e(TAG, "Failed to start ENCV-go daemon", e);
        }
    }

    private void copyBinaryFromAssets(File dest) throws Exception {
        try (InputStream is = getAssets().open(BINARY_NAME);
             FileOutputStream fos = new FileOutputStream(dest)) {
            byte[] buffer = new byte[8192];
            int len;
            while ((len = is.read(buffer)) != -1) {
                fos.write(buffer, 0, len);
            }
        }
        Log.i(TAG, "Copied Go binary to " + dest.getAbsolutePath());
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        if (goProcess != null && goProcess.isAlive()) {
            goProcess.destroy();
            Log.i(TAG, "ENCV-go daemon stopped");
        }
    }
}
