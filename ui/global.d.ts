declare module "*.css";
declare module "*.html";

declare namespace JSX {
  type Element = import("react").JSX.Element;
}
