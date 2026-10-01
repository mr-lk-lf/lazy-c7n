# Pruebas manuales (checklist del PM)

Todo contra un emulador local (Floci), **nunca** contra una cuenta real.
Tiempo aproximado: 20 minutos.

## Preparación (una vez)

```sh
cd lazy-c7n
python3 -m venv .venv && .venv/bin/pip install c7n
python3 -m venv .venv-emu && .venv-emu/bin/pip install 'moto[server]'
scripts/floci-dev.sh --reset      # Floci limpio, recursos de demo, abre lazyc7n
```

Recursos de demo: 3 buckets S3 (uno con `owner`), 3 instancias EC2 (2 con `owner`), 2 volúmenes EBS sueltos.

## 1. Policies (pantalla 1)

- [ ] Se ven 3 ficheros (`aws/ebs.yml`, `aws/ec2.yml`, `aws/s3.yml`) y 7 policies.
- [ ] Arriba a la derecha pone `c7n 0.9.52`.
- [ ] `delete`, `stop`, `terminate` salen en rojo; `periodic` en ámbar; `report` en gris.
- [ ] Al moverte con `j`/`k` la derecha muestra el resumen y el YAML con números de línea.
- [ ] `Enter` en un fichero lo pliega/despliega.
- [ ] `/` + `ec2-m` filtra; `Esc` quita el filtro.
- [ ] `Espacio` marca policies (●) y el título dice "N selected"; `Esc` las desmarca.
- [ ] `e` abre tu editor en la línea de la policy; al salir, se recarga.

## 2. Validate y dry-run

- [ ] Cursor en `aws/ec2.yml`, `v` → pantalla Jobs, termina con "valid".
- [ ] Rompe un fichero en el editor (p. ej. `resource: aws.nosuchthing`), `v` → "INVALID" en rojo y el error en la salida. Arréglalo después.
- [ ] Cursor en `aws/ec2.yml`, `d` → Jobs muestra el comando (con `--dryrun`) y el log en directo; al final "4 resources matched".
- [ ] Repite con `aws/s3.yml` y `aws/ebs.yml`.
- [ ] En Jobs, `x` sobre un job terminado dice "no running job selected".

## 3. Runs y Resources (pantallas 2 y 3)

- [ ] Runs lista los dry-runs (DRY, ✓, nº de policies y recursos), el más nuevo arriba.
- [ ] Arriba del detalle, el resumen: "N matches in X of Y policies", desglose por tipo (ec2, s3, ebs) y región; en rojo cuántos matches tocarían acciones destructivas (delete, terminate…), en ámbar los que cambiarían (tag, mark-for-op…).
- [ ] Los runs con acciones destructivas que encontraron algo llevan ⚠ en la lista.
- [ ] Haz dry-run de las 7 policies a la vez (marca los 3 ficheros con espacio y `d`) y comprueba que las cifras cuadran con la tabla.
- [ ] `Enter` → tabla de policies con estado `ok`, nº de recursos y las acciones coloreadas.
- [ ] `t` muestra el log de la policy; `t` otra vez vuelve.
- [ ] `Enter` sobre una policy → Resources: arriba una tabla con las columnas del report de c7n (en EC2: InstanceId, Name, InstanceType, LaunchTime, VpcId, IP); abajo la ficha del recurso (campos, edad, tags, "matched by").
- [ ] `t` cambia la ficha por el JSON coloreado y vuelta.
- [ ] Un run de un directorio `-output` también muestra la tabla (no necesita el fichero de policy original).
- [ ] `Esc` vuelve a Runs.

## 4. Confirmación live (lo más importante)

Intenta romperla. Nada de esto debe ejecutar nada:

- [ ] `R` sobre `ec2-terminate-untagged`: pantalla roja, comando SIN `--dryrun`, aviso "DESTRUCTIVE".
- [ ] Escribir mal el nombre + `Enter` → "does not match", sigue abierto.
- [ ] Medio nombre, el nombre en mayúsculas, `y`, `yes` → no pasa nada.
- [ ] `Esc` cierra; `Tab`, `q` y `1`-`5` no hacen nada con el diálogo abierto (se escriben como texto).
- [ ] Marca 3 policies y `R`: pide escribir `3` (no el nombre, no `ALL`).
- [ ] Con `s3-daily-owner-check` (periodic): tras el nombre, segundo paso "DEPLOYS INFRASTRUCTURE", hay que escribir `DEPLOY` (en mayúsculas).
- [ ] `R` desde Runs/Jobs dice "live runs start from the Policies screen".
- [ ] Una config con `[safety] confirm_live = "yes-no"` hace que lazyc7n no arranque y explique por qué.

Y ahora sí, contra Floci:

- [ ] `R` en `ec2-mark-stop`, escribe el nombre, `Enter` → job LIVE, "3 resources matched"; las instancias tienen ahora el tag `maid_status`.
- [ ] `R` en `s3-daily-owner-check` + `DEPLOY` → en Runs aparece con estado `deployed` y sin recursos.

## 5. Schema (pantalla 4)

- [ ] La primera vez tarda unos segundos ("running custodian schema --json…"); la siguiente es instantánea (caché).
- [ ] `/aws.ec2` + `Enter`, `Enter` → acciones (coloreadas) y filtros.
- [ ] `Enter` sobre una acción carga su ayuda; se ve también el JSON schema.

## 6. Contexto, versión y clasificación

- [ ] La barra inferior muestra a qué apunta: con `scripts/floci-dev.sh` debe decir `aws env keys · us-east-1 · endpoint localhost:4566`; con `AWS_PROFILE=prod` delante, `aws profile prod`.
- [ ] El diálogo live repite esa línea como "target: …".
- [ ] `lazyc7n version` muestra la versión de lazyc7n y la de custodian (o por qué no lo encuentra).
- [ ] En la config, `[safety.actions]` con `destructive = ["tag"]`: `s3-untagged-owner` sale en rojo y el diálogo live avisa de DESTRUCTIVE.
- [ ] `[safety.actions]` con `notify = ["delete"]`: lazyc7n no arranca y explica que una acción destructiva no se puede rebajar.

## 7. Otros

- [ ] `q` con un job en marcha pide confirmación; `q` otra vez sale.
- [ ] `scripts/floci-dev.sh --docker`: lo mismo pero con el backend docker (la barra inferior dice `docker: cloudcustodian/c7n`).
- [ ] `lazyc7n -prune` dice cuántos runs borró.
- [ ] `T` va cambiando de tema (lazyc7n, terminal, catppuccin, gruvbox, everforest, tokyonight, dracula); el rojo sigue siendo "peligro" y el verde "dry-run" en todos.
- [ ] `theme = "catppuccin"` + `appearance = "light"` en la config arranca con Catppuccin Latte.
